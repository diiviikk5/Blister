package httpdl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

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
