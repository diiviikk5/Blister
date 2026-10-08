// Package media downloads from streaming sites (YouTube, X, Instagram,
// TikTok, ...) and HLS playlists by driving yt-dlp, which Blister installs
// and keeps up to date on its own.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Tools finds (and if needed installs) the external binaries.
type Tools struct {
	Dir string // where Blister keeps its own copies
	// Overrides from settings; empty means auto.
	YtDlp  func() string
	Ffmpeg func() string
	Client func() *http.Client

	mu        sync.Mutex
	installed bool
	updated   time.Time
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func (t *Tools) find(override func() string, name string) string {
	if override != nil {
		if p := strings.TrimSpace(override()); p != "" {
			return p
		}
	}
	if local := filepath.Join(t.Dir, exe(name)); fileExists(local) {
		return local
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// FfmpegPath returns ffmpeg's location, or "" when it isn't available.
func (t *Tools) FfmpegPath() string { return t.find(t.Ffmpeg, "ffmpeg") }

// YtDlpPath returns yt-dlp's location without installing it.
func (t *Tools) YtDlpPath() string { return t.find(t.YtDlp, "yt-dlp") }

// EnsureYtDlp returns a usable yt-dlp, downloading the official release the
// first time it's needed.
func (t *Tools) EnsureYtDlp(ctx context.Context) (string, error) {
	if p := t.YtDlpPath(); p != "" {
		return p, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if p := t.YtDlpPath(); p != "" {
		return p, nil
	}
	url, ok := ytDlpRelease[runtime.GOOS]
	if !ok {
		return "", errors.New("install yt-dlp and add it to PATH to download from this site")
	}
	dst := filepath.Join(t.Dir, exe("yt-dlp"))
	if err := t.fetch(ctx, url, dst); err != nil {
		return "", fmt.Errorf("installing yt-dlp: %w", err)
	}
	t.installed = true
	t.updated = time.Now()
	return dst, nil
}

// MaybeUpdate refreshes Blister's own yt-dlp at most once a day; sites break
// old versions constantly.
func (t *Tools) MaybeUpdate(ctx context.Context) {
	t.mu.Lock()
	if time.Since(t.updated) < 24*time.Hour {
		t.mu.Unlock()
		return
	}
	t.updated = time.Now()
	t.mu.Unlock()
	local := filepath.Join(t.Dir, exe("yt-dlp"))
	if !fileExists(local) || t.YtDlpPath() != local {
		return // never touch a user-managed install
	}
	if st, err := os.Stat(local); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
		return
	}
	cmd := exec.CommandContext(ctx, local, "-U")
	hide(cmd)
	_ = cmd.Run()
	now := time.Now()
	_ = os.Chtimes(local, now, now)
}

var ytDlpRelease = map[string]string{
	"windows": "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe",
	"darwin":  "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos",
	"linux":   "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux",
}

func (t *Tools) fetch(ctx context.Context, url, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	c := http.DefaultClient
	if t.Client != nil {
		c = t.Client()
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
