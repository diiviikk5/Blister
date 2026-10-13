package bt

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"

	"github.com/diiviikk5/Blister/internal/config"
	"github.com/diiviikk5/Blister/internal/engine"
	"github.com/diiviikk5/Blister/internal/httpdl"
)

func TestAuthenticatedTorrentFile(t *testing.T) {
	_, _, path := seed(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=example" || r.Header.Get("Referer") != "https://example.com/page" || r.Header.Get("Authorization") != "Bearer example" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Range") != "" {
			t.Error("captured Range survived")
		}
		w.Write(data)
	}))
	defer srv.Close()
	c := &Client{}
	spec, err := c.spec(context.Background(), srv.URL, srv.Client(), httpdl.Request{Cookies: "session=example", Referer: "https://example.com/page", Headers: map[string]string{"Authorization": "Bearer example", "Range": "bytes=10-"}})
	if err != nil || spec == nil {
		t.Fatalf("authenticated torrent = %v, %v", spec, err)
	}
}

func TestPrivateTorrentKeepsOnlySuppliedTrackers(t *testing.T) {
	private := true
	info := metainfo.Info{Name: "private.bin", Length: 1, PieceLength: 16384, Pieces: make([]byte, 20), Private: &private}
	ib, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: ib, Announce: "http://127.0.0.1:1/private"}
	path := filepath.Join(t.TempDir(), "private.torrent")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := mi.Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	m, _, driver := setup(t, false)
	task, err := m.Add(engine.AddRequest{URL: path})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tk, _ := m.Get(task.ID)
		if tk.Status == engine.StatusError {
			t.Fatal(tk.Error)
		}
		if tk.Status == engine.StatusDownloading {
			// The driver has processed metadata by this point.
			client, err := driver.Client()
			if err != nil {
				t.Fatal(err)
			}
			active := client.cl.Torrents()
			if len(active) != 1 {
				t.Fatalf("active torrents = %d", len(active))
			}
			trackers := active[0].Metainfo().AnnounceList.DistinctValues()
			if len(trackers) != 1 || trackers[0] != mi.Announce {
				t.Fatalf("private torrent leaked to extra trackers: %v", trackers)
			}
			m.Pause(task.ID)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("private torrent did not load")
}

// seed creates a two-file torrent and starts a local seeder for it.
func seed(t *testing.T) (magnet string, files map[string][]byte, torrentPath string) {
	t.Helper()
	// Not t.TempDir: Windows keeps the seeder's handles briefly after Close,
	// which would fail the test during cleanup.
	base, err := os.MkdirTemp("", "blister-seed")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 20 && os.RemoveAll(base) != nil; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	})
	root := filepath.Join(base, "pack")
	_ = os.MkdirAll(root, 0o755)
	files = map[string][]byte{"a.bin": make([]byte, 3<<20), "sub/b.bin": make([]byte, 1<<20+17)}
	for name, b := range files {
		rand.New(rand.NewSource(int64(len(name)))).Read(b)
		p := filepath.Join(root, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	info := metainfo.Info{PieceLength: 256 << 10}
	if err := info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	ib, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: ib}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = filepath.Dir(root)
	stor := newStorage(filepath.Dir(root), storage.NewMapPieceCompletion())
	cfg.DefaultStorage = stor
	cfg.Seed = true
	cfg.NoDHT = true
	cfg.NoDefaultPortForwarding = true
	cfg.ListenPort = 0
	cfg.DisableIPv6 = true
	sc, err := torrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sc.Close(); stor.Close() })
	st, err := sc.AddTorrent(mi)
	if err != nil {
		t.Fatal(err)
	}
	<-st.GotInfo()
	if err := st.VerifyData(); err != nil {
		t.Fatal(err)
	}
	for i := 0; !st.Complete().Bool(); i++ {
		if i > 200 {
			t.Fatalf("seeder doesn't have the data (%d/%d)", st.BytesCompleted(), st.Length())
		}
		time.Sleep(10 * time.Millisecond)
	}
	port := sc.LocalPort()
	magnet = fmt.Sprintf("%s&x.pe=127.0.0.1:%d", mi.Magnet(nil, nil).String(), port)

	torrentPath = filepath.Join(t.TempDir(), "pack.torrent")
	f, _ := os.Create(torrentPath)
	_ = mi.Write(f)
	f.Close()
	return magnet, files, torrentPath
}

func setup(t *testing.T, seedAfter bool) (*engine.Manager, string, *Driver) {
	t.Helper()
	s := config.Defaults()
	s.DownloadDir = t.TempDir()
	s.CategorizeFolders = false
	s.EnableDHT = false
	s.EnableUPnP = false
	s.TorrentPort = 0
	s.SeedAfter = seedAfter
	dataDir := t.TempDir()
	m, err := engine.New(engine.Options{DataDir: dataDir, Settings: func() config.Settings { return s }})
	if err != nil {
		t.Fatal(err)
	}
	d := &Driver{New: func() (*Client, error) { return NewClient(dataDir, s, m.GlobalLimiter()) }}
	m.Register(engine.KindTorrent, d)
	t.Cleanup(func() { m.Close(); d.Close() })
	return m, s.DownloadDir, d
}

func wait(t *testing.T, m *engine.Manager, id string, want engine.Status) engine.Task {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		tk, _ := m.Get(id)
		if tk.Status == want {
			return tk
		}
		if tk.Status == engine.StatusError {
			t.Fatalf("torrent failed: %s", tk.Error)
		}
		time.Sleep(50 * time.Millisecond)
	}
	tk, _ := m.Get(id)
	t.Fatalf("timed out in %s (%d/%d)", tk.Status, tk.Done, tk.Size)
	return tk
}

func TestMagnetDownload(t *testing.T) {
	magnet, files, _ := seed(t)
	m, dl, _ := setup(t, false)
	task, err := m.Add(engine.AddRequest{URL: magnet})
	if err != nil {
		t.Fatal(err)
	}
	if task.Kind != engine.KindTorrent {
		t.Fatalf("kind %s", task.Kind)
	}
	done := wait(t, m, task.ID, engine.StatusCompleted)
	if done.Name != "pack" || len(done.Torrent.Files) != 2 {
		t.Fatalf("bad metadata: %+v", done.Torrent)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dl, "pack", filepath.FromSlash(name)))
		if err != nil {
			_ = filepath.Walk(dl, func(p string, _ os.FileInfo, _ error) error { t.Log(p); return nil })
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			diff := -1
			for i := range got {
				if i >= len(want) || got[i] != want[i] {
					diff = i
					break
				}
			}
			t.Fatalf("%s corrupt: got %d bytes want %d, first diff at %d", name, len(got), len(want), diff)
		}
	}
}

func TestTorrentFileAndSeeding(t *testing.T) {
	_, _, tp := seed(t)
	m, _, _ := setup(t, true)
	task, err := m.Add(engine.AddRequest{URL: tp})
	if err != nil {
		t.Fatal(err)
	}
	if task.Kind != engine.KindTorrent {
		t.Fatalf("kind %s for %s", task.Kind, tp)
	}
	// Without peers we can't finish, but metadata from the file is instant.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tk, _ := m.Get(task.ID)
		if tk.Torrent != nil && len(tk.Torrent.Files) == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("file list never loaded from .torrent")
}

func TestPieceMapShape(t *testing.T) {
	magnet, _, _ := seed(t)
	m, _, _ := setup(t, false)
	task, _ := m.Add(engine.AddRequest{URL: magnet})
	done := wait(t, m, task.ID, engine.StatusCompleted)
	if pm := done.Torrent.PieceMap; pm == "" || bytes.ContainsAny([]byte(pm), "012345678") {
		t.Fatalf("finished torrent piece map not full: %q", pm)
	}
}
