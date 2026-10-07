package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/diiviikk5/Blister/internal/config"
)

func blob(n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(7)).Read(b)
	return b
}

// slowServer serves data with ranges, throttled so tests can pause mid-way.
func slowServer(data []byte, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
}

type recorder struct {
	mu     sync.Mutex
	events map[string]int
}

func (r *recorder) emit(ev string, _ any) {
	r.mu.Lock()
	r.events[ev]++
	r.mu.Unlock()
}

func (r *recorder) count(ev string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.events[ev]
}

func newManager(t *testing.T, dataDir, dlDir string) (*Manager, *recorder) {
	t.Helper()
	rec := &recorder{events: map[string]int{}}
	s := config.Defaults()
	s.DownloadDir = dlDir
	s.CategorizeFolders = false
	m, err := New(Options{DataDir: dataDir, Settings: func() config.Settings { return s }, Emit: rec.emit})
	if err != nil {
		t.Fatal(err)
	}
	m.Register(KindHTTP, HTTPDriver{})
	return m, rec
}

func waitStatus(t *testing.T, m *Manager, id string, want Status, timeout time.Duration) Task {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if tk, ok := m.Get(id); ok && tk.Status == want {
			return tk
		}
		time.Sleep(10 * time.Millisecond)
	}
	tk, _ := m.Get(id)
	t.Fatalf("task never reached %s (now %s, err %q)", want, tk.Status, tk.Error)
	return tk
}

func TestDownloadCompletes(t *testing.T) {
	data := blob(6 << 20)
	srv := slowServer(data, 0)
	defer srv.Close()
	dl := t.TempDir()
	m, rec := newManager(t, t.TempDir(), dl)
	defer m.Close()

	sum := sha256.Sum256(data)
	task, err := m.Add(AddRequest{URL: srv.URL + "/dir/file.bin", Checksum: "sha256:" + hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	done := waitStatus(t, m, task.ID, StatusCompleted, 15*time.Second)
	got, err := os.ReadFile(filepath.Join(dl, "file.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("content mismatch")
	}
	if done.Verified == nil || !*done.Verified {
		t.Fatal("checksum not verified")
	}
	if rec.count(EvCompleted) != 1 {
		t.Fatalf("completed events: %d", rec.count(EvCompleted))
	}
}

func TestChecksumMismatchFails(t *testing.T) {
	srv := slowServer(blob(1<<20), 0)
	defer srv.Close()
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	task, _ := m.Add(AddRequest{URL: srv.URL + "/x.bin", Checksum: "sha256:" + string(bytes.Repeat([]byte("0"), 64))})
	tk := waitStatus(t, m, task.ID, StatusError, 10*time.Second)
	if tk.Error == "" {
		t.Fatal("no error message")
	}
}

func TestPauseResumeAndPersist(t *testing.T) {
	data := blob(48 << 20)
	srv := slowServer(data, 0)
	defer srv.Close()
	dataDir, dl := t.TempDir(), t.TempDir()
	m, _ := newManager(t, dataDir, dl)

	task, _ := m.Add(AddRequest{URL: srv.URL + "/big.bin", SpeedLimit: 8 << 20, Connections: 4})
	waitStatus(t, m, task.ID, StatusDownloading, 5*time.Second)
	time.Sleep(600 * time.Millisecond)
	if err := m.Pause(task.ID); err != nil {
		t.Fatal(err)
	}
	paused := waitStatus(t, m, task.ID, StatusPaused, 5*time.Second)
	if paused.Done <= 0 || paused.Done >= int64(len(data)) {
		t.Fatalf("unexpected progress at pause: %d", paused.Done)
	}
	if len(paused.Segments) == 0 {
		t.Fatal("no resume state saved")
	}
	m.Close()

	// Restart the app: state must survive and resume where it left off.
	m2, _ := newManager(t, dataDir, dl)
	defer m2.Close()
	reloaded, ok := m2.Get(task.ID)
	if !ok || reloaded.Status != StatusPaused || reloaded.Done != paused.Done {
		t.Fatalf("state not persisted: %+v", reloaded)
	}
	_ = m2.Tune(task.ID, 8, 0) // lift the limit
	if err := m2.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m2, task.ID, StatusCompleted, 30*time.Second)
	got, _ := os.ReadFile(filepath.Join(dl, "big.bin"))
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file corrupt")
	}
}

func TestQueueRespectsMaxActive(t *testing.T) {
	srv := slowServer(blob(2<<20), 0)
	defer srv.Close()
	rec := &recorder{events: map[string]int{}}
	s := config.Defaults()
	s.DownloadDir = t.TempDir()
	s.CategorizeFolders = false
	s.MaxActive = 1
	m, _ := New(Options{Settings: func() config.Settings { return s }, Emit: rec.emit})
	m.Register(KindHTTP, HTTPDriver{})
	defer m.Close()

	var ids []string
	for i := 0; i < 3; i++ {
		tk, _ := m.Add(AddRequest{URL: srv.URL + "/f.bin", SpeedLimit: 1 << 20})
		ids = append(ids, tk.ID)
	}
	time.Sleep(300 * time.Millisecond)
	running := 0
	for _, tk := range m.List() {
		if tk.Status.Active() {
			running++
		}
	}
	if running != 1 {
		t.Fatalf("running %d tasks with MaxActive=1", running)
	}
	for _, id := range ids {
		waitStatus(t, m, id, StatusCompleted, 20*time.Second)
	}
	// Same URL thrice must not overwrite each other.
	names := map[string]bool{}
	for _, tk := range m.List() {
		names[tk.Name] = true
	}
	if len(names) != 3 {
		t.Fatalf("names collided: %v", names)
	}
}

func TestRemoveDeletesFiles(t *testing.T) {
	srv := slowServer(blob(1<<20), 0)
	defer srv.Close()
	dl := t.TempDir()
	m, rec := newManager(t, t.TempDir(), dl)
	defer m.Close()
	tk, _ := m.Add(AddRequest{URL: srv.URL + "/gone.bin"})
	waitStatus(t, m, tk.ID, StatusCompleted, 10*time.Second)
	if err := m.Remove(tk.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dl, "gone.bin")); !os.IsNotExist(err) {
		t.Fatal("file not deleted")
	}
	if len(m.List()) != 0 || rec.count(EvRemoved) != 1 {
		t.Fatal("task not removed")
	}
}

func TestDetectKind(t *testing.T) {
	cases := map[string]Kind{
		"magnet:?xt=urn:btih:abc":                     KindTorrent,
		"https://x.org/ubuntu.iso.torrent":            KindTorrent,
		"https://cdn.x/live/master.m3u8?token=1":      KindHLS,
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ": KindMedia,
		"https://youtu.be/dQw4w9WgXcQ":                KindMedia,
		"https://x.com/user/status/1":                 KindMedia,
		"https://files.example.com/a.zip":             KindHTTP,
	}
	for in, want := range cases {
		if got := DetectKind(in); got != want {
			t.Errorf("DetectKind(%q)=%s want %s", in, got, want)
		}
	}
}

func TestInWindow(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 1, 1, h, m, 0, 0, time.Local) }
	if !InWindow(at(3, 0), "", "") {
		t.Fatal("empty window should always allow")
	}
	if !InWindow(at(1, 30), "23:00", "07:00") || InWindow(at(12, 0), "23:00", "07:00") {
		t.Fatal("wrapping window wrong")
	}
	if !InWindow(at(9, 0), "08:00", "17:00") || InWindow(at(17, 0), "08:00", "17:00") {
		t.Fatal("daytime window wrong")
	}
}

func TestParseChecksum(t *testing.T) {
	if a, _, err := ParseChecksum("d41d8cd98f00b204e9800998ecf8427e"); err != nil || a != "md5" {
		t.Fatalf("md5 inference: %s %v", a, err)
	}
	if _, _, err := ParseChecksum("sha3:abcd"); err == nil {
		t.Fatal("expected unsupported algo error")
	}
}
