// Package bridge is the local HTTP endpoint the browser extension talks to.
// It only listens on loopback and every write needs the shared token.
package bridge

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Capture is a download the browser handed to Blister.
type Capture struct {
	URL       string            `json:"url"`
	URLs      []string          `json:"urls,omitempty"`
	FileName  string            `json:"filename,omitempty"`
	Referer   string            `json:"referer,omitempty"`
	Cookies   string            `json:"cookies,omitempty"`
	UserAgent string            `json:"userAgent,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Size      int64             `json:"size,omitempty"`
	// Media asks for the streaming-site downloader (video pages).
	Media bool `json:"media,omitempty"`
}

// Server serves the bridge API.
type Server struct {
	Version string
	Token   func() string
	OnAdd   func(Capture) error

	mu  sync.Mutex
	srv *http.Server
	ln  net.Listener
}

// Start listens on 127.0.0.1:port, replacing any previous listener.
func (s *Server) Start(port int) error {
	s.Stop()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("browser bridge: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", s.ping)
	mux.HandleFunc("/add", s.add)
	srv := &http.Server{Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	s.mu.Lock()
	s.srv, s.ln = srv, ln
	s.mu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	return nil
}

// Stop closes the listener.
func (s *Server) Stop() {
	s.mu.Lock()
	srv := s.srv
	s.srv, s.ln = nil, nil
	s.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

// Addr is the bound address, for tests.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// cors lets extension pages call us; web pages are rejected by the token.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if strings.HasPrefix(origin, "chrome-extension://") || strings.HasPrefix(origin, "moz-extension://") ||
			strings.HasPrefix(origin, "extension://") || strings.HasPrefix(origin, "safari-web-extension://") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Blister-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) authed(r *http.Request) bool {
	want := s.Token()
	got := r.Header.Get("X-Blister-Token")
	return want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"app": "blister", "version": s.Version, "authorized": s.authed(r)})
}

func (s *Server) add(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	if !s.authed(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "pair the extension with Blister first"})
		return
	}
	var c Capture
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if c.URL == "" && len(c.URLs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errors.New("no link").Error()})
		return
	}
	if err := s.OnAdd(c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
