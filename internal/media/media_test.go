package media

import (
	"slices"
	"strings"
	"testing"

	"github.com/diiviikk5/Blister/internal/engine"
)

func argAfter(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func TestBuildArgsFormats(t *testing.T) {
	base := engine.Task{URL: "https://youtu.be/x", Dir: `C:\dl`, Media: &engine.MediaInfo{}}

	a := buildArgs(base, "ffmpeg.exe", 0)
	if argAfter(a, "-f") != "bv*+ba/b" || argAfter(a, "--ffmpeg-location") != "ffmpeg.exe" {
		t.Fatalf("best with ffmpeg: %v", a)
	}
	if a[len(a)-1] != base.URL || a[len(a)-2] != "--" {
		t.Fatal("url must come last after --")
	}

	a = buildArgs(base, "", 0)
	if argAfter(a, "-f") != "b" {
		t.Fatalf("no ffmpeg should pick a muxed format: %v", a)
	}

	hd := base
	hd.Media = &engine.MediaInfo{Format: "720"}
	if got := argAfter(buildArgs(hd, "ff", 0), "-S"); !strings.HasPrefix(got, "res:720") {
		t.Fatalf("height sort = %q", got)
	}
	if got := argAfter(buildArgs(hd, "", 0), "-f"); got != "b[height<=720]/b" {
		t.Fatalf("height no ffmpeg = %q", got)
	}

	au := base
	au.Media = &engine.MediaInfo{AudioOnly: true}
	if !slices.Contains(buildArgs(au, "ff", 0), "-x") {
		t.Fatal("audio with ffmpeg should extract")
	}

	lim := buildArgs(base, "", 500000)
	if argAfter(lim, "-r") != "500000" {
		t.Fatal("rate limit not passed")
	}

	named := base
	named.Name, named.NameFixed = "clip.mp4", true
	if o := argAfter(buildArgs(named, "", 0), "-o"); !strings.HasSuffix(o, "clip.%(ext)s") {
		t.Fatalf("fixed name output = %q", o)
	}
}

func TestNum(t *testing.T) {
	if num("NA") != 0 || num("1024") != 1024 || num("12.5") != 12 {
		t.Fatal("num parsing")
	}
}

func TestHumanize(t *testing.T) {
	if !strings.Contains(humanize("[youtube] x: Private video. Sign in if you've been granted access"), "signed-in") {
		t.Fatal("private video message")
	}
}
