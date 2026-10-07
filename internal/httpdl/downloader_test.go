package httpdl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func payload(n int) []byte {
	b := make([]byte, n)
	rand.New(rand.NewSource(42)).Read(b)
	return b
}

func rangeServer(data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="blob.bin"`)
		http.ServeContent(w, r, "blob.bin", time.Time{}, bytes.NewReader(data))
	}))
}

func client(t *testing.T) *http.Client {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func checkFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(want) {
		t.Fatalf("content mismatch: got %d bytes want %d", len(got), len(want))
	}
}

func TestProbe(t *testing.T) {
	data := payload(5 << 20)
	srv := rangeServer(data)
	defer srv.Close()
	p, err := ProbeURL(context.Background(), client(t), Request{URL: srv.URL + "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Size != int64(len(data)) || !p.Resumable || p.FileName != "blob.bin" {
		t.Fatalf("bad probe: %+v", p)
	}
}

func TestMultiConnectionDownload(t *testing.T) {
	data := payload(24<<20 + 12345)
	srv := rangeServer(data)
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{
		Client: client(t), Request: Request{URL: srv.URL}, Path: out,
		Size: int64(len(data)), Resumable: true, Connections: 16,
	})
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.Written() != int64(len(data)) {
		t.Fatalf("written %d want %d", d.Written(), len(data))
	}
	checkFile(t, out, data)
}

func TestResumeFromSnapshot(t *testing.T) {
	data := payload(16 << 20)
	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &countingWriter{ResponseWriter: w, n: &served}
		http.ServeContent(cw, r, "x", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	opts := Options{
		Client: client(t), Request: Request{URL: srv.URL}, Path: out,
		Size: int64(len(data)), Resumable: true, Connections: 4,
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := New(opts)
	go func() {
		for d.Written() < 6<<20 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	if err := d.Run(ctx); err == nil {
		t.Fatal("expected cancellation")
	}
	snap := d.Snapshot()
	had := d.Written()

	served.Store(0)
	opts.Segments = snap
	d2 := New(opts)
	if d2.Written() != had {
		t.Fatalf("resume restored %d bytes, had %d", d2.Written(), had)
	}
	if err := d2.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
	// The second run must not re-download what was already on disk
	// (allow slack for in-flight buffers discarded at cancel).
	if got, max := served.Load(), int64(len(data))-had+int64(opts.Connections)*bufSize*2; got > max {
		t.Fatalf("resume re-fetched too much: %d > %d", got, max)
	}
}

type countingWriter struct {
	http.ResponseWriter
	n *atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.n.Add(int64(n))
	return n, err
}

func TestNoRangeServer(t *testing.T) {
	data := payload(3 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	c := client(t)
	p, err := ProbeURL(context.Background(), c, Request{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if p.Resumable {
		t.Fatal("server without ranges reported resumable")
	}
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{Client: c, Request: Request{URL: srv.URL}, Path: out, Size: p.Size, Resumable: p.Resumable, Connections: 8})
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
}

func TestUnknownSize(t *testing.T) {
	data := payload(2<<20 + 7)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
		_, _ = io.Copy(w, bytes.NewReader(data))
	}))
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{Client: client(t), Request: Request{URL: srv.URL}, Path: out, Size: -1})
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
}

// flaky drops every few responses mid-stream to exercise retries.
func TestFlakyServerRecovers(t *testing.T) {
	data := payload(12 << 20)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1)%3 == 0 {
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		http.ServeContent(w, r, "x", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{
		Client: client(t), Request: Request{URL: srv.URL}, Path: out,
		Size: int64(len(data)), Resumable: true, Connections: 8, MaxRetries: 20,
	})
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
}

func TestNotFoundFailsFast(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := ProbeURL(context.Background(), client(t), Request{URL: srv.URL}); err == nil {
		t.Fatal("expected 404 error")
	}
}

func TestSetConnectionsWhileRunning(t *testing.T) {
	data := payload(32 << 20)
	srv := rangeServer(data)
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{
		Client: client(t), Request: Request{URL: srv.URL}, Path: out,
		Size: int64(len(data)), Resumable: true, Connections: 2,
	})
	go func() {
		time.Sleep(5 * time.Millisecond)
		d.SetConnections(12)
		time.Sleep(5 * time.Millisecond)
		d.SetConnections(1)
	}()
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
}

func TestFileName(t *testing.T) {
	cases := map[string]string{
		`attachment; filename="report.pdf"`:               "report.pdf",
		`attachment; filename*=UTF-8''na%C3%AFve%20x.txt`: "naïve x.txt",
		`inline; filename=a:b?.zip`:                       "a_b_.zip",
	}
	for disp, want := range cases {
		if got := FileName(disp, nil, ""); got != want {
			t.Errorf("FileName(%q)=%q want %q", disp, got, want)
		}
	}
}
