package httpdl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/diiviikk5/Blister/internal/ratelimit"
)

func TestSanitizeReservedAndUnicodeNames(t *testing.T) {
	for _, name := range []string{"COM9.zip", "LPT8.txt", "CON.tar.gz", "aux .txt"} {
		if !strings.HasPrefix(Sanitize(name), "_") {
			t.Errorf("reserved name survived: %q", name)
		}
	}
	name := Sanitize(strings.Repeat("界", 100) + ".zip")
	if !utf8.ValidString(name) || len(name) > 200 || !strings.HasSuffix(name, ".zip") {
		t.Fatalf("invalid shortened name: %q", name)
	}
}

func TestProbeDoesNotTrustAcceptRanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Write([]byte("whole file"))
	}))
	defer srv.Close()
	p, err := ProbeURL(context.Background(), client(t), Request{URL: srv.URL})
	if err != nil || p.Resumable || p.Size != 10 {
		t.Fatalf("probe = %+v, error = %v", p, err)
	}
}

func TestMissingOrTruncatedPartialRestarts(t *testing.T) {
	data := payload(256 << 10)
	srv := rangeServer(data)
	defer srv.Close()
	for _, mode := range []string{"missing", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "f.part")
			if mode == "truncated" {
				if err := os.WriteFile(out, data[:10], 0600); err != nil {
					t.Fatal(err)
				}
			}
			d := New(Options{Client: client(t), Request: Request{URL: srv.URL}, Path: out, Size: int64(len(data)), Resumable: true,
				Segments: []Segment{{Start: 0, End: int64(len(data)), Pos: int64(len(data))}}})
			if err := d.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			checkFile(t, out, data)
		})
	}
}

func TestThrottlingIsNotAStall(t *testing.T) {
	data := payload(192 << 10)
	srv := rangeServer(data)
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "f.part")
	d := New(Options{Client: client(t), Request: Request{URL: srv.URL}, Path: out, Size: int64(len(data)), Resumable: true,
		Connections: 1, StallTimeout: 150 * time.Millisecond, Limiters: []*ratelimit.Limiter{ratelimit.New(64 << 10)}})
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := d.Run(ctx); err != nil {
		t.Fatal(err)
	}
	checkFile(t, out, data)
}

func TestChangedIfRangeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Range") != `"version1"` {
			t.Error("missing file validator")
		}
		fmt.Fprint(w, "new!")
	}))
	defer srv.Close()
	d := New(Options{Client: client(t), Request: Request{URL: srv.URL}, Path: filepath.Join(t.TempDir(), "f.part"), Size: 4,
		Resumable: true, Validator: `"version1"`, Connections: 1})
	if err := d.Run(context.Background()); !errors.Is(err, ErrRemoteChanged) {
		t.Fatalf("error = %v", err)
	}
	if d.Written() != 0 {
		t.Fatal("changed file was written")
	}
}

func TestProbeEmptyFile(t *testing.T) {
	srv := rangeServer(nil)
	defer srv.Close()
	p, err := ProbeURL(context.Background(), client(t), Request{URL: srv.URL})
	if err != nil || p.Size != 0 || p.Resumable {
		t.Fatalf("empty probe = %+v, error = %v", p, err)
	}
}

func TestRejectedRangeResponses(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		rangeHeader string
		want        error
	}{
		{"ignored at offset zero", 200, "", ErrRangeIgnored},
		{"missing range", 206, "", ErrInvalidRange},
		{"wrong offset", 206, "bytes 1-4/4", ErrInvalidRange},
		{"changed total", 206, "bytes 0-3/8", ErrInvalidRange},
		{"short range", 206, "bytes 0-1/4", ErrInvalidRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Range", tc.rangeHeader)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, "data")
			}))
			defer srv.Close()
			d := New(Options{Client: client(t), Request: Request{URL: srv.URL}, Path: filepath.Join(t.TempDir(), "f.part"), Size: 4, Resumable: true, Connections: 1})
			if err := d.Run(context.Background()); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if d.Written() != 0 {
				t.Fatal("unvalidated response was written")
			}
		})
	}
}

func TestCapturedHeadersCannotControlTransfer(t *testing.T) {
	r := Request{URL: "https://example.com", Headers: map[string]string{
		"Range": "bytes=100-", "If-Range": "old", "If-None-Match": "cached", "If-Modified-Since": "yesterday", "Accept-Encoding": "gzip", "Authorization": "Bearer example",
	}}
	req, err := r.build(context.Background(), http.MethodGet, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"} {
		if req.Header.Get(h) != "" {
			t.Errorf("captured %s survived", h)
		}
	}
	if req.Header.Get("Accept-Encoding") != "identity" || req.Header.Get("Authorization") != "Bearer example" {
		t.Fatal("encoding or authentication header incorrect")
	}
}
