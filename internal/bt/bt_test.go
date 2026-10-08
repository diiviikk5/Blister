package bt

import (
	"bytes"
	"fmt"
	"math/rand"
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
)

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
	task, _ := m.Add(engine.AddRequest{URL: tp})
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
