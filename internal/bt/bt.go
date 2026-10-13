// Package bt runs BitTorrent downloads (magnets and .torrent files) on top of
// anacrolix/torrent and plugs them into the engine as a Driver.
package bt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"

	"github.com/diiviikk5/Blister/internal/config"
	"github.com/diiviikk5/Blister/internal/httpdl"
	"github.com/diiviikk5/Blister/internal/ratelimit"
)

// Extra public trackers appended to every torrent. Magnets often ship with
// none or dead ones; these dramatically improve time-to-first-peer.
var extraTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.stealth.si:80/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"https://tracker.tamersunion.org:443/announce",
}

// Client is a long-lived BitTorrent client shared by every torrent task.
type Client struct {
	cl         *torrent.Client
	completion storage.PieceCompletion
	up         *ratelimit.Limiter

	mu       sync.Mutex
	storages map[string]storage.ClientImplCloser
}

// NewClient starts the BitTorrent client. down is the shared global download
// limiter so torrents and HTTP downloads obey one combined cap.
func NewClient(dataDir string, s config.Settings, down *ratelimit.Limiter) (*Client, error) {
	btDir := filepath.Join(dataDir, "torrent")
	if err := os.MkdirAll(btDir, 0o755); err != nil {
		return nil, err
	}
	completion, err := storage.NewBoltPieceCompletion(btDir)
	if err != nil {
		// A locked/corrupt db shouldn't disable torrents entirely.
		completion = storage.NewMapPieceCompletion()
	}

	up := ratelimit.New(s.UploadLimit)
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = btDir
	cfg.DefaultStorage = newStorage(btDir, completionNoClose{completion})
	cfg.ListenPort = s.TorrentPort
	cfg.Seed = true
	cfg.NoDHT = !s.EnableDHT
	cfg.NoDefaultPortForwarding = !s.EnableUPnP
	cfg.DownloadRateLimiter = down.Raw()
	cfg.UploadRateLimiter = up.Raw()
	cfg.EstablishedConnsPerTorrent = 80
	cfg.HalfOpenConnsPerTorrent = 40
	cfg.TotalHalfOpenConns = 200
	cfg.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{
		Preferred:        s.EncryptPeers,
		RequirePreferred: false,
	}
	cfg.Slogger = slog.New(slog.NewTextHandler(io.Discard, nil))

	cl, err := torrent.NewClient(cfg)
	for attempt := 0; err != nil && addressInUse(err) && attempt < 3; attempt++ {
		// TCP and UDP must share a port. Even a kernel-selected TCP port
		// can already be occupied for UDP, so resample that case as well.
		cfg.ListenPort = 0
		cl, err = torrent.NewClient(cfg)
	}
	if err != nil {
		completion.Close()
		return nil, fmt.Errorf("start torrent client: %w", err)
	}
	return &Client{cl: cl, completion: completion, up: up, storages: map[string]storage.ClientImplCloser{}}, nil
}

func addressInUse(err error) bool {
	// WinSock WSAEADDRINUSE differs from Go's portable EADDRINUSE sentinel.
	return errors.Is(err, syscall.EADDRINUSE) || (runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10048)))
}

// SetUploadLimit changes the global upload cap.
func (c *Client) SetUploadLimit(bps int64) { c.up.SetRate(bps) }

// Close stops the client and flushes piece completion state.
func (c *Client) Close() {
	c.cl.Close()
	c.mu.Lock()
	for _, s := range c.storages {
		_ = s.Close()
	}
	c.mu.Unlock()
	_ = c.completion.Close()
}

func (c *Client) storageFor(dir string) storage.ClientImpl {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.storages[dir]; ok {
		return s
	}
	s := newStorage(dir, completionNoClose{c.completion})
	c.storages[dir] = s
	return s
}

// completionNoClose lets many storages share one completion database while
// only the Client closes it.
type completionNoClose struct{ storage.PieceCompletion }

func (completionNoClose) Close() error { return nil }

// Add loads a magnet link, a .torrent URL or a local .torrent file and
// returns the live torrent, saving data into dir.
func (c *Client) Add(ctx context.Context, source, dir string, httpc *http.Client, requests ...httpdl.Request) (*torrent.Torrent, error) {
	spec, err := c.spec(ctx, source, httpc, requests...)
	if err != nil {
		return nil, err
	}
	spec.Storage = c.storageFor(dir)
	t, _, err := c.cl.AddTorrentSpec(spec)
	return t, err
}

func (c *Client) spec(ctx context.Context, source string, httpc *http.Client, requests ...httpdl.Request) (*torrent.TorrentSpec, error) {
	src := strings.TrimSpace(source)
	switch {
	case strings.HasPrefix(strings.ToLower(src), "magnet:"):
		return torrent.TorrentSpecFromMagnetUri(src)
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		request := httpdl.Request{URL: src}
		if len(requests) > 0 {
			request = requests[0]
			request.URL = src
		}
		req, err := request.NewRequest(ctx, http.MethodGet)
		if err != nil {
			return nil, err
		}
		resp, err := httpc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("fetch .torrent: %s", resp.Status)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		if err != nil {
			return nil, err
		}
		mi, err := metainfo.Load(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("not a valid .torrent file: %w", err)
		}
		return torrent.TorrentSpecFromMetaInfoErr(mi)
	default:
		path := src
		if strings.HasPrefix(strings.ToLower(src), "file://") {
			u, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("invalid torrent file URL: %w", err)
			}
			path = u.Path
			if runtime.GOOS == "windows" {
				if len(path) > 3 && path[0] == '/' && path[2] == ':' {
					path = path[1:]
				}
				if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
					path = `\\` + u.Host + path
				}
			} else if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
				return nil, errors.New("torrent file URL must point to this computer")
			}
			path = filepath.FromSlash(path)
		}
		mi, err := metainfo.LoadFromFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("torrent file not found: %s", path)
			}
			return nil, fmt.Errorf("not a valid .torrent file: %w", err)
		}
		return torrent.TorrentSpecFromMetaInfoErr(mi)
	}
}
