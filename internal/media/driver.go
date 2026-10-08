package media

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/diiviikk5/Blister/internal/engine"
)

// Driver runs media and HLS tasks through yt-dlp.
type Driver struct {
	Tools *Tools
}

// Run implements engine.Driver.
func (d *Driver) Run(ctx context.Context, j *engine.Job) error {
	ytdlp, err := d.Tools.EnsureYtDlp(ctx)
	if err != nil {
		return err
	}
	go d.Tools.MaybeUpdate(context.Background())

	task := j.Task()
	ffmpeg := d.Tools.FfmpegPath()
	args := buildArgs(task, ffmpeg, rateFor(j))
	cmd := exec.CommandContext(ctx, ytdlp, args...)
	hide(cmd)
	cmd.Cancel = func() error { return killTree(cmd) }

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}

	j.Update(func(t *engine.Task) { t.Resumable = true })
	p := &progress{j: j}
	var errLines []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p.consume(stdout) }()
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "ERROR:") {
				mu.Lock()
				errLines = append(errLines, strings.TrimSpace(strings.TrimPrefix(line, "ERROR:")))
				mu.Unlock()
			}
		}
	}()
	wg.Wait()
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if werr != nil {
		if len(errLines) > 0 {
			return errors.New(humanize(errLines[len(errLines)-1]))
		}
		return fmt.Errorf("yt-dlp failed: %w", werr)
	}
	if p.final != "" {
		j.Update(func(t *engine.Task) {
			t.Dir = filepath.Dir(p.final)
			t.Name = filepath.Base(p.final)
			if t.Media != nil {
				t.Media.Stage = ""
			}
		})
	}
	return nil
}

// rateFor is the tightest active limit, since yt-dlp can't share our limiter.
func rateFor(j *engine.Job) int64 {
	var r int64
	for _, l := range j.Limiters() {
		if v := l.Rate(); v > 0 && (r == 0 || v < r) {
			r = v
		}
	}
	return r
}

func buildArgs(t engine.Task, ffmpeg string, rate int64) []string {
	audio := t.Media != nil && t.Media.AudioOnly
	format := ""
	if t.Media != nil {
		format = strings.TrimSpace(t.Media.Format)
	}
	out := filepath.Join(t.Dir, "%(title).150B [%(id)s].%(ext)s")
	if t.NameFixed && t.Name != "" {
		base := strings.TrimSuffix(t.Name, filepath.Ext(t.Name))
		out = filepath.Join(t.Dir, base+".%(ext)s")
	}
	args := []string{
		"--newline", "--no-colors", "--no-playlist", "--no-mtime",
		"--windows-filenames", "--continue", "--no-quiet",
		"--progress-template", "download:BLP %(progress.status)s %(progress.downloaded_bytes)s %(progress.total_bytes)s %(progress.total_bytes_estimate)s %(info.format_id)s",
		"--no-simulate",
		"--print", "before_dl:BLMETA %(.{title,thumbnail,id})j",
		"--print", "after_move:BLPATH %(filepath)s",
		"-o", out,
	}
	if n := t.Connections; n > 1 {
		args = append(args, "-N", strconv.Itoa(min(n, 16)))
	} else {
		args = append(args, "-N", "8")
	}
	if rate > 0 {
		args = append(args, "-r", strconv.FormatInt(rate, 10))
	}
	if ua := t.Request.UserAgent; ua != "" {
		args = append(args, "--user-agent", ua)
	}
	for k, v := range t.Request.Headers {
		args = append(args, "--add-header", k+":"+v)
	}
	if ffmpeg != "" {
		args = append(args, "--ffmpeg-location", ffmpeg)
	}
	switch {
	case audio && ffmpeg != "":
		args = append(args, "-f", "ba/b", "-x", "--audio-format", "mp3", "--audio-quality", "0")
	case audio:
		args = append(args, "-f", "ba[ext=m4a]/ba/b")
	case format != "" && format != "best" && !isHeight(format):
		args = append(args, "-f", format)
	case ffmpeg != "":
		sort := "res,ext:mp4:m4a"
		if isHeight(format) {
			sort = "res:" + format + ",ext:mp4:m4a"
		}
		args = append(args, "-f", "bv*+ba/b", "-S", sort, "--merge-output-format", "mp4/mkv")
	default:
		// No ffmpeg: only formats that already contain audio and video.
		if isHeight(format) {
			args = append(args, "-f", "b[height<="+format+"]/b")
		} else {
			args = append(args, "-f", "b")
		}
	}
	return append(args, "--", t.URL)
}

func isHeight(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 144 && n <= 4320
}

// progress turns yt-dlp's output into task telemetry. A video+audio download
// is two consecutive transfers; their sizes add up.
type progress struct {
	j     *engine.Job
	final string

	format     string
	prevDone   int64 // bytes of finished stages
	prevSize   int64
	curDone    int64
	curSize    int64
	stageCount int
}

func (p *progress) consume(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		p.line(sc.Text())
	}
}

func (p *progress) line(line string) {
	switch {
	case strings.HasPrefix(line, "BLP "):
		p.onProgress(strings.Fields(line)[1:])
	case strings.HasPrefix(line, "BLMETA "):
		var meta struct {
			Title     string `json:"title"`
			Thumbnail string `json:"thumbnail"`
			ID        string `json:"id"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "BLMETA ")), &meta) == nil {
			p.j.Update(func(t *engine.Task) {
				if t.Media == nil {
					t.Media = &engine.MediaInfo{}
				}
				t.Media.Title = meta.Title
				t.Media.Thumbnail = meta.Thumbnail
				if !t.NameFixed && meta.Title != "" {
					t.Name = meta.Title
				}
			})
			p.j.SetStatus(engine.StatusDownloading)
		}
	case strings.HasPrefix(line, "BLPATH "):
		p.final = strings.TrimSpace(strings.TrimPrefix(line, "BLPATH "))
	case strings.HasPrefix(line, "[Merger]"):
		p.stage("merging")
	case strings.HasPrefix(line, "[ExtractAudio]"):
		p.stage("converting")
	case strings.HasPrefix(line, "[FixupM3u8]"), strings.HasPrefix(line, "[FixupM4a]"):
		p.stage("fixing")
	}
}

func (p *progress) stage(s string) {
	p.j.Update(func(t *engine.Task) {
		if t.Media != nil {
			t.Media.Stage = s
		}
	})
}

func num(s string) int64 {
	if s == "NA" || s == "None" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f)
}

func (p *progress) onProgress(f []string) {
	if len(f) < 4 {
		return
	}
	status, done := f[0], num(f[1])
	size := num(f[2])
	if size <= 0 {
		size = num(f[3])
	}
	format := ""
	if len(f) > 4 {
		format = f[4]
	}
	if format != p.format && p.format != "" {
		// Next stream (e.g. audio after video).
		p.prevDone += max(p.curSize, p.curDone)
		p.prevSize += max(p.curSize, p.curDone)
		p.curDone, p.curSize = 0, 0
	}
	if p.format == "" || format != p.format {
		p.stageCount++
		stage := "video"
		if p.stageCount > 1 {
			stage = "audio"
		}
		p.stage(stage)
	}
	p.format = format
	p.curDone, p.curSize = done, size
	if status == "finished" && size <= 0 {
		p.curSize = done
	}
	total := p.prevSize + p.curSize
	if p.curSize <= 0 {
		total = -1
	}
	p.j.SetDone(p.prevDone + p.curDone)
	p.j.SetSize(total)
}

func humanize(msg string) string {
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "unsupported url"):
		return "This site isn't supported for video downloads"
	case strings.Contains(low, "private video"), strings.Contains(low, "sign in"), strings.Contains(low, "login required"):
		return "This video needs a signed-in account to download"
	case strings.Contains(low, "video unavailable"), strings.Contains(low, "has been removed"):
		return "The video is unavailable or was removed"
	case strings.Contains(low, "requested format is not available"):
		return "That quality isn't available for this video"
	case strings.Contains(low, "http error 403"):
		return "The site refused the download (403). Try again, or update yt-dlp in Settings"
	}
	return msg
}
