// Package config holds user settings and where Blister keeps its files.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Settings are every user-tunable option. JSON names are shared with the UI.
type Settings struct {
	// Where finished files land.
	DownloadDir string `json:"downloadDir"`
	// Sort files into Videos/, Music/, Archives/... under DownloadDir.
	CategorizeFolders bool `json:"categorizeFolders"`
	// How many downloads run at once; the rest wait in the queue.
	MaxActive int `json:"maxActive"`
	// Default parallel connections per HTTP download.
	Connections int `json:"connections"`
	// Global download cap in bytes/s (0 = unlimited).
	SpeedLimit int64 `json:"speedLimit"`
	// Global upload cap for torrents in bytes/s (0 = unlimited).
	UploadLimit int64 `json:"uploadLimit"`
	// Retries per connection before a download is marked failed.
	MaxRetries int `json:"maxRetries"`

	Proxy       string `json:"proxy"`
	UserAgent   string `json:"userAgent"`
	InsecureTLS bool   `json:"insecureTls"`

	// Torrents
	TorrentPort  int     `json:"torrentPort"`
	SeedAfter    bool    `json:"seedAfter"`
	SeedRatio    float64 `json:"seedRatio"`
	EnableDHT    bool    `json:"enableDht"`
	EnableUPnP   bool    `json:"enableUpnp"`
	EncryptPeers bool    `json:"encryptPeers"`

	// External tools for streaming sites. Empty = look on PATH.
	YtDlpPath  string `json:"ytDlpPath"`
	FfmpegPath string `json:"ffmpegPath"`

	// Integration
	ClipboardWatch bool   `json:"clipboardWatch"`
	BrowserPort    int    `json:"browserPort"`
	BrowserToken   string `json:"browserToken"`

	// UX
	Theme         string `json:"theme"`   // "dark" | "light" | "system"
	Accent        string `json:"accent"`  // hex colour
	Density       string `json:"density"` // "comfortable" | "compact"
	Notifications bool   `json:"notifications"`
	Sounds        bool   `json:"sounds"`
	ConfirmDelete bool   `json:"confirmDelete"`
	StartOnBoot   bool   `json:"startOnBoot"`
	CloseToTray   bool   `json:"closeToTray"`

	// Scheduler: only download inside this window (HH:MM, empty = always).
	ScheduleStart string `json:"scheduleStart"`
	ScheduleEnd   string `json:"scheduleEnd"`

	// Rice: the UI's whole look as an opaque JSON document owned by the
	// frontend (colours, shape, type, layout, effects, custom CSS), plus the
	// user's saved themes. Go only stores them.
	Rice        string `json:"rice"`
	RiceLibrary string `json:"riceLibrary"`
	// Native window backdrop: "none", "mica", "acrylic" or "tabbed".
	// Applied at startup.
	WindowEffect string `json:"windowEffect"`
}

// Defaults are sensible out-of-the-box settings.
func Defaults() Settings {
	home, _ := os.UserHomeDir()
	return Settings{
		DownloadDir:       filepath.Join(home, "Downloads", "Blister"),
		CategorizeFolders: true,
		MaxActive:         4,
		Connections:       16,
		MaxRetries:        10,
		TorrentPort:       42069,
		SeedAfter:         true,
		SeedRatio:         1.0,
		EnableDHT:         true,
		EnableUPnP:        true,
		EncryptPeers:      true,
		ClipboardWatch:    true,
		BrowserPort:       7323,
		Theme:             "dark",
		Accent:            "#ffd60a",
		Density:           "comfortable",
		Notifications:     true,
		Sounds:            true,
		ConfirmDelete:     true,
		CloseToTray:       false,
	}
}

// Normalize clamps values into safe ranges.
func (s *Settings) Normalize() {
	d := Defaults()
	switch s.WindowEffect {
	case "none", "mica", "acrylic", "tabbed":
	default:
		s.WindowEffect = "none"
	}
	if s.DownloadDir == "" {
		s.DownloadDir = d.DownloadDir
	}
	s.MaxActive = clamp(s.MaxActive, 1, 32, d.MaxActive)
	s.Connections = clamp(s.Connections, 1, 64, d.Connections)
	s.MaxRetries = clamp(s.MaxRetries, 1, 100, d.MaxRetries)
	if s.TorrentPort <= 0 || s.TorrentPort > 65535 {
		s.TorrentPort = d.TorrentPort
	}
	if s.BrowserPort <= 0 || s.BrowserPort > 65535 {
		s.BrowserPort = d.BrowserPort
	}
	if s.SpeedLimit < 0 {
		s.SpeedLimit = 0
	}
	if s.UploadLimit < 0 {
		s.UploadLimit = 0
	}
	if s.SeedRatio < 0 {
		s.SeedRatio = 0
	}
	if s.Theme == "" {
		s.Theme = d.Theme
	}
	if s.Accent == "" {
		s.Accent = d.Accent
	}
	if s.Density == "" {
		s.Density = d.Density
	}
}

func clamp(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Dir is Blister's data directory (%APPDATA%\Blister on Windows).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Blister")
	return dir, os.MkdirAll(dir, 0o755)
}

// Store loads and saves settings to a JSON file.
type Store struct {
	path string
	mu   sync.RWMutex
	s    Settings
}

// Open reads settings from dir/settings.json, creating defaults if absent.
func Open(dir string) (*Store, error) {
	st := &Store{path: filepath.Join(dir, "settings.json"), s: Defaults()}
	b, err := os.ReadFile(st.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, &st.s); err != nil {
			// Corrupt settings shouldn't brick the app; keep defaults but
			// preserve the bad file for inspection.
			_ = os.Rename(st.path, st.path+".bad")
			st.s = Defaults()
		}
	}
	st.s.Normalize()
	return st, nil
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.s
}

// Set replaces settings and persists them.
func (st *Store) Set(s Settings) (Settings, error) {
	s.Normalize()
	st.mu.Lock()
	st.s = s
	st.mu.Unlock()
	return s, WriteJSON(st.path, s)
}

// WriteJSON atomically writes v as indented JSON.
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
