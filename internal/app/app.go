// Package app is the layer between the Wails window and the engine: every
// exported method on App is callable from the UI.
package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/diiviikk5/Blister/internal/bridge"
	"github.com/diiviikk5/Blister/internal/bt"
	"github.com/diiviikk5/Blister/internal/category"
	"github.com/diiviikk5/Blister/internal/config"
	"github.com/diiviikk5/Blister/internal/engine"
	"github.com/diiviikk5/Blister/internal/httpdl"
	"github.com/diiviikk5/Blister/internal/linkgrab"
	"github.com/diiviikk5/Blister/internal/media"
	"github.com/diiviikk5/Blister/internal/rules"
)

// Version is set at build time with -ldflags "-X .../app.Version=...".
var Version = "0.1.0-dev"

// Events the UI listens for, besides the engine's own.
const (
	EvSettings  = "settings:changed"
	EvExternal  = "external:links" // links from the browser, CLI or a second launch
	EvClipboard = "clipboard:links"
)

// App is bound to the window.
type App struct {
	ctx     context.Context
	dataDir string
	store   *config.Store
	m       *engine.Manager
	bt      *bt.Driver
	tools   *media.Tools
	bridge  *bridge.Server

	mu    sync.Mutex
	ready bool
	// Window geometry to restore after mini bar mode.
	restore *[4]int
	pending []External
	notify  bool
}

// External is a link handed to Blister from outside the window.
type External struct {
	URLs     []string       `json:"urls"`
	Request  httpdl.Request `json:"request"`
	FileName string         `json:"fileName,omitempty"`
	Media    bool           `json:"media,omitempty"`
	Source   string         `json:"source"` // "browser" | "launch" | "clipboard"
}

// New prepares the engine. The window isn't up yet, so nothing is emitted
// until Startup.
func New() (*App, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	store, err := config.Open(dir)
	if err != nil {
		return nil, err
	}
	a := &App{dataDir: dir, store: store}
	if s := store.Get(); s.BrowserToken == "" {
		s.BrowserToken = newToken()
		_, _ = store.Set(s)
	}

	a.m, err = engine.New(engine.Options{DataDir: dir, Settings: store.Get, Emit: a.emitEngine})
	if err != nil {
		return nil, err
	}
	a.tools = &media.Tools{
		Dir:    filepath.Join(dir, "tools"),
		YtDlp:  func() string { return store.Get().YtDlpPath },
		Ffmpeg: func() string { return store.Get().FfmpegPath },
		Client: a.m.Client,
	}
	a.bt = &bt.Driver{New: func() (*bt.Client, error) { return bt.NewClient(dir, store.Get(), a.m.GlobalLimiter()) }}
	md := &media.Driver{Tools: a.tools}
	a.m.Register(engine.KindHTTP, engine.HTTPDriver{})
	a.m.Register(engine.KindTorrent, a.bt)
	a.m.Register(engine.KindMedia, md)
	a.m.Register(engine.KindHLS, md)

	a.bridge = &bridge.Server{
		Version: Version,
		Token:   func() string { return store.Get().BrowserToken },
		OnAdd:   a.fromBrowser,
	}
	return a, nil
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Startup runs once the window exists.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	s := a.store.Get()
	if err := a.bridge.Start(s.BrowserPort); err != nil {
		runtime.LogWarning(ctx, err.Error())
	}
	if runtime.InitializeNotifications(ctx) == nil {
		a.notify = runtime.IsNotificationAvailable(ctx)
	}
	go a.watchClipboard(ctx)
}

// DomReady marks the UI as able to receive events and flushes anything that
// arrived before it.
func (a *App) DomReady(ctx context.Context) {
	a.mu.Lock()
	a.ready = true
	pending := a.pending
	a.pending = nil
	a.mu.Unlock()
	for _, p := range pending {
		runtime.EventsEmit(ctx, EvExternal, p)
	}
}

// Shutdown stops downloads cleanly so they resume next launch.
func (a *App) Shutdown(context.Context) {
	a.bridge.Stop()
	a.m.Close()
	a.bt.Close()
}

func (a *App) emit(ev string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, ev, data)
	}
}

func (a *App) emitEngine(ev string, data any) {
	a.emit(ev, data)
	if t, ok := data.(engine.Task); ok && ev == engine.EvCompleted {
		go a.runHooks(t)
	}
	if !a.notify || !a.store.Get().Notifications {
		return
	}
	t, ok := data.(engine.Task)
	if !ok {
		return
	}
	switch ev {
	case engine.EvCompleted:
		_ = runtime.SendNotification(a.ctx, runtime.NotificationOptions{ID: t.ID, Title: "Download complete", Body: t.Name})
	case engine.EvFailed:
		_ = runtime.SendNotification(a.ctx, runtime.NotificationOptions{ID: t.ID, Title: "Download failed", Body: t.Name + ": " + t.Error})
	}
}

// external queues links for the UI's add dialog, or sends them right away.
func (a *App) external(e External) {
	a.mu.Lock()
	if !a.ready {
		a.pending = append(a.pending, e)
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.emit(EvExternal, e)
	if a.ctx != nil {
		runtime.WindowUnminimise(a.ctx)
		runtime.WindowShow(a.ctx)
	}
}

func (a *App) fromBrowser(c bridge.Capture) error {
	urls := c.URLs
	if c.URL != "" {
		urls = append([]string{c.URL}, urls...)
	}
	a.external(External{
		URLs:     urls,
		FileName: c.FileName,
		Media:    c.Media,
		Source:   "browser",
		Request: httpdl.Request{
			Referer:   c.Referer,
			Cookies:   c.Cookies,
			UserAgent: c.UserAgent,
			Headers:   c.Headers,
		},
	})
	return nil
}

// HandleArgs takes command-line arguments (magnets, .torrent files, links)
// from this launch or a second one.
func (a *App) HandleArgs(args []string) {
	var urls []string
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if linkgrab.Valid(arg) || linkgrab.IsMagnet(arg) || strings.EqualFold(filepath.Ext(arg), ".torrent") {
			urls = append(urls, arg)
		}
	}
	if len(urls) > 0 {
		a.external(External{URLs: urls, Source: "launch"})
	}
}

// watchClipboard offers links the user copies anywhere.
func (a *App) watchClipboard(ctx context.Context) {
	last, _ := runtime.ClipboardGetText(ctx)
	t := time.NewTicker(900 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !a.store.Get().ClipboardWatch {
			continue
		}
		text, err := runtime.ClipboardGetText(ctx)
		if err != nil || text == last {
			continue
		}
		last = text
		if len(text) > 200_000 {
			continue
		}
		if links := linkgrab.Extract(text); len(links) > 0 {
			a.emit(EvClipboard, links)
		}
	}
}

// ---------------------------------------------------------------- UI API

// Boot is everything the UI needs on first paint.
type Boot struct {
	Version  string          `json:"version"`
	Tasks    []engine.Task   `json:"tasks"`
	Settings config.Settings `json:"settings"`
	Tools    ToolStatus      `json:"tools"`
	DataDir  string          `json:"dataDir"`
}

// ToolStatus reports optional helpers for streaming sites.
type ToolStatus struct {
	YtDlp  string `json:"ytDlp"`
	Ffmpeg string `json:"ffmpeg"`
}

// Boot returns the initial state.
func (a *App) Boot() Boot {
	return Boot{
		Version:  Version,
		Tasks:    a.m.List(),
		Settings: a.store.Get(),
		Tools:    a.Tools(),
		DataDir:  a.dataDir,
	}
}

// Tools reports which helper binaries are available.
func (a *App) Tools() ToolStatus {
	return ToolStatus{YtDlp: a.tools.YtDlpPath(), Ffmpeg: a.tools.FfmpegPath()}
}

// InstallYtDlp fetches yt-dlp now instead of on first video download.
func (a *App) InstallYtDlp() (ToolStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	_, err := a.tools.EnsureYtDlp(ctx)
	return a.Tools(), err
}

// Link is one parsed link for the add dialog.
type Link struct {
	URL  string      `json:"url"`
	Kind engine.Kind `json:"kind"`
	Name string      `json:"name"`
}

// Parse pulls every link out of pasted text and expands batch patterns
// like file[01-20].zip.
func (a *App) Parse(text string) ([]Link, error) {
	var out []Link
	seen := map[string]bool{}
	add := func(u string) {
		if seen[u] {
			return
		}
		seen[u] = true
		k := engine.DetectKind(u)
		out = append(out, Link{URL: u, Kind: k, Name: engine.GuessName(u, k)})
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if engine.IsLocalPath(line) && strings.EqualFold(filepath.Ext(line), ".torrent") {
			add(line)
			continue
		}
		for _, l := range linkgrab.Extract(line) {
			exp, err := linkgrab.Expand(l)
			if err != nil {
				return nil, err
			}
			if len(exp) > 5000 {
				return nil, errors.New("that pattern expands to more than 5000 links")
			}
			for _, u := range exp {
				add(u)
			}
		}
	}
	return out, nil
}

// ProbeResult previews an HTTP download before it's added.
type ProbeResult struct {
	Name      string            `json:"name"`
	Size      int64             `json:"size"`
	Resumable bool              `json:"resumable"`
	Category  category.Category `json:"category"`
	Type      string            `json:"type"`
	Error     string            `json:"error,omitempty"`
}

// Probe asks the server about a file (name, size, resume support).
func (a *App) Probe(url string, req httpdl.Request) ProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req.URL = url
	if req.UserAgent == "" {
		req.UserAgent = a.store.Get().UserAgent
	}
	p, err := httpdl.ProbeURL(ctx, a.m.Client(), req)
	if err != nil {
		return ProbeResult{Size: -1, Error: err.Error()}
	}
	return ProbeResult{
		Name:      p.FileName,
		Size:      p.Size,
		Resumable: p.Resumable,
		Category:  category.Detect(p.FileName, p.ContentType),
		Type:      p.ContentType,
	}
}

// applyRules lets the user's automation shape a new download. Explicit
// choices from the add dialog win over rules, except connections, which the
// dialog always fills with the global default.
func (a *App) applyRules(r *engine.AddRequest) {
	s := a.store.Get()
	if len(s.Rules) == 0 {
		return
	}
	kind := r.Kind
	if kind == "" {
		kind = engine.DetectKind(r.URL)
	}
	name := r.Name
	if name == "" {
		name = engine.GuessName(r.URL, kind)
	}
	e := rules.Evaluate(s.Rules, rules.Item{URL: r.URL, Name: name, Kind: string(kind), Size: r.Size})
	if len(e.Matched) == 0 {
		return
	}
	if r.Dir == "" && e.Dir != "" {
		r.Dir = e.Dir
	}
	if e.Connections > 0 && (r.Connections == 0 || r.Connections == s.Connections) {
		r.Connections = e.Connections
	}
	if r.SpeedLimit == 0 && e.SpeedLimit > 0 {
		r.SpeedLimit = e.SpeedLimit
	}
	r.Paused = r.Paused || e.Paused
	r.Tags = append(r.Tags, e.Tags...)
	for _, id := range e.Matched {
		r.Tags = append(r.Tags, "rule:"+id)
	}
}

// runHooks starts the global and per-rule completion commands for t.
func (a *App) runHooks(t engine.Task) {
	s := a.store.Get()
	var cmds []string
	if strings.TrimSpace(s.OnComplete) != "" {
		cmds = append(cmds, s.OnComplete)
	}
	for _, tag := range t.Tags {
		id, ok := strings.CutPrefix(tag, "rule:")
		if !ok {
			continue
		}
		for _, r := range s.Rules {
			if r.ID == id && strings.TrimSpace(r.Run) != "" {
				cmds = append(cmds, r.Run)
			}
		}
	}
	vars := map[string]string{
		"path": t.Path(), "name": t.Name, "dir": t.Dir, "url": t.URL, "size": fmt.Sprint(t.Size),
	}
	for _, c := range cmds {
		if err := runShell(rules.Expand(c, vars), t.Dir); err != nil && a.ctx != nil {
			runtime.LogWarningf(a.ctx, "hook failed: %v", err)
		}
	}
}

// Add queues downloads.
func (a *App) Add(reqs []engine.AddRequest) ([]engine.Task, error) {
	var out []engine.Task
	var errs []error
	for _, r := range reqs {
		a.applyRules(&r)
		t, err := a.m.Add(r)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.URL, err))
			continue
		}
		out = append(out, *t)
	}
	return out, errors.Join(errs...)
}

func each(ids []string, fn func(string) error) error {
	var errs []error
	for _, id := range ids {
		errs = append(errs, fn(id))
	}
	return errors.Join(errs...)
}

// Pause stops downloads, keeping their progress.
func (a *App) Pause(ids []string) error { return each(ids, a.m.Pause) }

// Resume queues paused or failed downloads again.
func (a *App) Resume(ids []string) error { return each(ids, a.m.Resume) }

// Restart throws away progress and starts over.
func (a *App) Restart(ids []string) error { return each(ids, a.m.Restart) }

// Remove deletes downloads from the list, optionally with their files.
func (a *App) Remove(ids []string, deleteFiles bool) error {
	return each(ids, func(id string) error { return a.m.Remove(id, deleteFiles) })
}

// PauseAll pauses everything.
func (a *App) PauseAll() { a.m.PauseAll() }

// ResumeAll resumes everything paused.
func (a *App) ResumeAll() { a.m.ResumeAll() }

// ClearCompleted removes finished downloads from the list (files stay).
func (a *App) ClearCompleted() { a.m.ClearCompleted() }

// Move reorders the queue.
func (a *App) Move(id string, index int) error { return a.m.Move(id, index) }

// Tune changes a running download's connections and speed cap.
func (a *App) Tune(id string, connections int, speedLimit int64) error {
	return a.m.Tune(id, connections, speedLimit)
}

// SelectFiles picks which files of a torrent to download.
func (a *App) SelectFiles(id string, selected []bool) error { return a.m.SelectFiles(id, selected) }

// SetURL replaces an expired or broken link and keeps progress.
func (a *App) SetURL(id, url string) error { return a.m.SetURL(id, url) }

// Rename changes a download's file name.
func (a *App) Rename(id, name string) error { return a.m.Rename(id, name) }

// Open launches a finished file with its default app.
func (a *App) Open(id string) error {
	t, ok := a.m.Get(id)
	if !ok {
		return errors.New("download not found")
	}
	p := t.Path()
	if _, err := os.Stat(p); err != nil {
		return errors.New("the file was moved or deleted")
	}
	return openPath(p)
}

// Reveal shows a download in the file manager.
func (a *App) Reveal(id string) error {
	t, ok := a.m.Get(id)
	if !ok {
		return errors.New("download not found")
	}
	p := t.Path()
	if _, err := os.Stat(p); err != nil {
		if _, err := os.Stat(t.PartPath()); err == nil {
			p = t.PartPath()
		} else {
			return openPath(t.Dir)
		}
	}
	return revealPath(p)
}

// OpenDownloads opens the main download folder.
func (a *App) OpenDownloads() error {
	dir := a.store.Get().DownloadDir
	_ = os.MkdirAll(dir, 0o755)
	return openPath(dir)
}

// OpenURL opens a link in the default browser.
func (a *App) OpenURL(url string) { runtime.BrowserOpenURL(a.ctx, url) }

// Settings returns current settings.
func (a *App) Settings() config.Settings { return a.store.Get() }

// SaveSettings validates, stores and applies settings.
func (a *App) SaveSettings(s config.Settings) (config.Settings, error) {
	old := a.store.Get()
	if s.BrowserToken == "" {
		s.BrowserToken = old.BrowserToken
	}
	s, err := a.store.Set(s)
	if err != nil {
		return old, err
	}
	if err := a.m.ApplySettings(); err != nil {
		return s, err
	}
	if c, err := a.bt.Client(); err == nil && c != nil && old.UploadLimit != s.UploadLimit {
		c.SetUploadLimit(s.UploadLimit)
	}
	if old.BrowserPort != s.BrowserPort {
		if err := a.bridge.Start(s.BrowserPort); err != nil {
			return s, err
		}
	}
	if old.StartOnBoot != s.StartOnBoot {
		if err := setStartOnBoot(s.StartOnBoot); err != nil {
			return s, err
		}
	}
	a.emit(EvSettings, s)
	return s, nil
}

// ResetToken issues a new browser pairing token.
func (a *App) ResetToken() (config.Settings, error) {
	s := a.store.Get()
	s.BrowserToken = newToken()
	return a.SaveSettings(s)
}

// SetSpeedLimit is the quick throttle in the status bar.
func (a *App) SetSpeedLimit(bps int64) (config.Settings, error) {
	s := a.store.Get()
	s.SpeedLimit = bps
	return a.SaveSettings(s)
}

// PickFolder shows a folder picker.
func (a *App) PickFolder(current string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a folder", DefaultDirectory: current, CanCreateDirectories: true,
	})
}

// PickImage lets the user choose a wallpaper and returns it as a data URL,
// so themes stay self-contained and shareable.
func (a *App) PickImage() (string, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose a wallpaper",
		Filters: []runtime.FileFilter{{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.gif"}},
	})
	if err != nil || p == "" {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.Size() > 8<<20 {
		return "", errors.New("pick an image under 8 MB")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	mime := http.DetectContentType(b)
	if !strings.HasPrefix(mime, "image/") {
		return "", errors.New("that file isn't an image")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// Relaunch restarts Blister (window effects only apply at startup).
func (a *App) Relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// The new copy waits for this one to exit; otherwise the single-instance
	// lock would hand it straight back to us just before we quit.
	if err := exec.Command(exe, fmt.Sprintf("--wait-for=%d", os.Getpid())).Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

// PickTorrents shows a .torrent file picker.
func (a *App) PickTorrents() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Open torrent files",
		Filters: []runtime.FileFilter{{DisplayName: "Torrent files (*.torrent)", Pattern: "*.torrent"}},
	})
}

// ShowWindow brings the window back (second launch, tray-style hide).
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// Close hides the window when "close to tray" is on, otherwise quits.
func (a *App) Close() {
	if a.store.Get().CloseToTray {
		runtime.WindowHide(a.ctx)
		return
	}
	runtime.Quit(a.ctx)
}

// SetMini turns the window into a slim always-on-top bar (or back).
func (a *App) SetMini(on bool) {
	ctx := a.ctx
	if ctx == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if on {
		if a.restore == nil {
			w, h := runtime.WindowGetSize(ctx)
			x, y := runtime.WindowGetPosition(ctx)
			a.restore = &[4]int{w, h, x, y}
		}
		if runtime.WindowIsMaximised(ctx) {
			runtime.WindowUnmaximise(ctx)
		}
		const mw, mh = 820, 52
		runtime.WindowSetMinSize(ctx, 420, mh)
		runtime.WindowSetSize(ctx, mw, mh)
		if screens, err := runtime.ScreenGetAll(ctx); err == nil {
			for _, sc := range screens {
				if sc.IsCurrent {
					runtime.WindowSetPosition(ctx, (sc.Size.Width-mw)/2, 14)
				}
			}
		}
		runtime.WindowSetAlwaysOnTop(ctx, true)
		return
	}
	runtime.WindowSetAlwaysOnTop(ctx, false)
	runtime.WindowSetMinSize(ctx, 880, 560)
	if r := a.restore; r != nil {
		runtime.WindowSetSize(ctx, max(r[0], 880), max(r[1], 560))
		runtime.WindowSetPosition(ctx, r[2], r[3])
	} else {
		runtime.WindowSetSize(ctx, 1240, 780)
		runtime.WindowCenter(ctx)
	}
	a.restore = nil
}

// Quit closes Blister.
func (a *App) Quit() { runtime.Quit(a.ctx) }
