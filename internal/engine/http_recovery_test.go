package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diiviikk5/Blister/internal/httpdl"
)

func TestDownloadDoesNotLeakAuthenticationToRedirectHost(t *testing.T) {
	data := blob(128 << 10)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials leaked to redirect host")
		}
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=example" {
			t.Error("original host lost cookies")
		}
		http.Redirect(w, r, strings.Replace(destination.URL, "127.0.0.1", "localhost", 1)+"/file.bin", http.StatusFound)
	}))
	defer source.Close()
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	task, err := m.Add(AddRequest{URL: source.URL + "/file.bin", Request: httpdl.Request{Cookies: "session=example", Headers: map[string]string{"Authorization": "Bearer example"}}})
	if err != nil {
		t.Fatal(err)
	}
	done := waitStatus(t, m, task.ID, StatusCompleted, 10*time.Second)
	got, err := os.ReadFile(done.Path())
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("redirect download failed: %v", err)
	}
}

func TestRetryChecksumFailureFetchesAgain(t *testing.T) {
	want := blob(256 << 10)
	var repaired atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, len(want))
		if repaired.Load() {
			data = want
		}
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	sum := sha256.Sum256(want)
	task, err := m.Add(AddRequest{URL: srv.URL + "/file.bin", Checksum: "sha256:" + hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, task.ID, StatusError, 5*time.Second)
	repaired.Store(true)
	if err := m.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	done := waitStatus(t, m, task.ID, StatusCompleted, 5*time.Second)
	got, err := os.ReadFile(done.Path())
	if err != nil || !bytes.Equal(got, want) || done.Verified == nil || !*done.Verified {
		t.Fatalf("retry failed: %+v, %v", done, err)
	}
}

func TestHTTPRangeFallback(t *testing.T) {
	data := blob(4 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-0" {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(data)))
			w.WriteHeader(206)
			w.Write(data[:1])
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	task, err := m.Add(AddRequest{URL: srv.URL + "/file.bin"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitStatus(t, m, task.ID, StatusCompleted, 10*time.Second)
	got, err := os.ReadFile(done.Path())
	if err != nil || !bytes.Equal(got, data) || done.Resumable {
		t.Fatalf("fallback failed: %+v, %v", done, err)
	}
}

func TestHTTPRecoversPartialAndChangedDate(t *testing.T) {
	data := blob(256 << 10)
	modified := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file.bin", modified, bytes.NewReader(data))
	}))
	defer srv.Close()
	for _, mode := range []string{"missing", "truncated", "changed date"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := newManager(t, t.TempDir(), t.TempDir())
			defer m.Close()
			task, err := m.Add(AddRequest{URL: srv.URL + "/file.bin", Paused: true})
			if err != nil {
				t.Fatal(err)
			}
			if mode != "missing" {
				partial := data[:10]
				if mode == "changed date" {
					partial = make([]byte, len(data))
				}
				if err := os.WriteFile(task.PartPath(), partial, 0600); err != nil {
					t.Fatal(err)
				}
			}
			m.mu.Lock()
			stored := m.tasks[task.ID]
			stored.Size, stored.Done = int64(len(data)), int64(len(data))
			stored.Segments = []httpdl.Segment{{Start: 0, End: stored.Size, Pos: stored.Size}}
			stored.LastModified = modified.Add(-time.Hour).Format(http.TimeFormat)
			if mode != "changed date" {
				stored.LastModified = modified.Format(http.TimeFormat)
			}
			m.mu.Unlock()
			if err := m.Resume(task.ID); err != nil {
				t.Fatal(err)
			}
			done := waitStatus(t, m, task.ID, StatusCompleted, 10*time.Second)
			got, err := os.ReadFile(done.Path())
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("recovered file corrupt: %v", err)
			}
		})
	}
}

func TestHTTPEmptyFileCompletes(t *testing.T) {
	srv := slowServer(nil, 0)
	defer srv.Close()
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	task, err := m.Add(AddRequest{URL: srv.URL + "/empty.bin"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitStatus(t, m, task.ID, StatusCompleted, 5*time.Second)
	st, err := os.Stat(done.Path())
	if err != nil || st.Size() != 0 || done.Done != 0 {
		t.Fatalf("empty file failed: %+v, %v", done, err)
	}
}
