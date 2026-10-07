// Package engine is Blister's download manager: a persistent queue of tasks,
// a scheduler that runs them through protocol drivers, and live telemetry.
package engine

import (
	"time"

	"github.com/diiviikk5/Blister/internal/category"
	"github.com/diiviikk5/Blister/internal/httpdl"
)

// Kind selects the driver that runs a task.
type Kind string

const (
	KindHTTP    Kind = "http"
	KindTorrent Kind = "torrent"
	KindHLS     Kind = "hls"
	KindMedia   Kind = "media" // yt-dlp backed (YouTube & co.)
)

// Status is a task's lifecycle state.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusStarting    Status = "starting"
	StatusDownloading Status = "downloading"
	StatusPaused      Status = "paused"
	StatusSeeding     Status = "seeding"
	StatusCompleted   Status = "completed"
	StatusError       Status = "error"
)

// Active reports whether the status holds a running slot.
func (s Status) Active() bool {
	return s == StatusStarting || s == StatusDownloading
}

// Task is one download. It is serialised to disk and sent to the UI as-is.
type Task struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	URL  string `json:"url"`
	Name string `json:"name"`
	// NameFixed means the user chose the name; don't replace it with the
	// server's suggestion.
	NameFixed bool              `json:"nameFixed,omitempty"`
	Dir       string            `json:"dir"`
	Category  category.Category `json:"category"`
	Status    Status            `json:"status"`
	Error     string            `json:"error,omitempty"`

	Size      int64 `json:"size"` // -1 unknown
	Done      int64 `json:"done"`
	Speed     int64 `json:"speed"`     // bytes/s, smoothed
	UpSpeed   int64 `json:"upSpeed"`   // torrents
	Uploaded  int64 `json:"uploaded"`  // torrents
	ETA       int64 `json:"eta"`       // seconds, -1 unknown
	Conns     int   `json:"conns"`     // live connections / peers
	Seeds     int   `json:"seeds"`     // torrents
	Resumable bool  `json:"resumable"` // can pause without losing data

	// Per-task tuning.
	Connections int   `json:"connections"`
	SpeedLimit  int64 `json:"speedLimit"`
	// Lower runs first.
	Priority int `json:"priority"`

	Request  httpdl.Request   `json:"request"`
	Segments []httpdl.Segment `json:"segments,omitempty"`
	ETag     string           `json:"etag,omitempty"`
	// Checksum to verify on completion, "sha256:abcd..." (md5/sha1 too).
	Checksum string `json:"checksum,omitempty"`
	Verified *bool  `json:"verified,omitempty"`

	// Torrent / media extras.
	Torrent *TorrentInfo `json:"torrent,omitempty"`
	Media   *MediaInfo   `json:"media,omitempty"`

	CreatedAt   time.Time `json:"createdAt"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// TorrentInfo is torrent-specific state.
type TorrentInfo struct {
	InfoHash string        `json:"infoHash"`
	Files    []TorrentFile `json:"files,omitempty"`
	Ratio    float64       `json:"ratio"`
	Pieces   int           `json:"pieces"`
	// Compact piece completion map for the UI ("1" have, "0" missing),
	// down-sampled to at most 512 cells.
	PieceMap string `json:"pieceMap,omitempty"`
}

// TorrentFile is one file inside a torrent.
type TorrentFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Done     int64  `json:"done"`
	Selected bool   `json:"selected"`
}

// MediaInfo is state for streaming-site downloads.
type MediaInfo struct {
	Format    string `json:"format"` // yt-dlp format selector or "best"
	AudioOnly bool   `json:"audioOnly"`
	Title     string `json:"title,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Stage     string `json:"stage,omitempty"` // "video", "audio", "merging"
}

// Path is the final location of the finished file (or torrent root).
func (t *Task) Path() string {
	return joinPath(t.Dir, t.Name)
}

// PartPath is where in-progress bytes are written.
func (t *Task) PartPath() string {
	return t.Path() + ".part"
}

// Progress in [0,1], or -1 when the size is unknown.
func (t *Task) Progress() float64 {
	if t.Size <= 0 {
		if t.Status == StatusCompleted {
			return 1
		}
		return -1
	}
	return float64(t.Done) / float64(t.Size)
}
