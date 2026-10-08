package bridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestBridge(t *testing.T) {
	var got Capture
	s := &Server{Version: "test", Token: func() string { return "secret" }, OnAdd: func(c Capture) error { got = c; return nil }}
	if err := s.Start(0); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	base := "http://" + s.Addr()

	post := func(token string, body any) int {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, base+"/add", bytes.NewReader(b))
		req.Header.Set("Origin", "chrome-extension://abc")
		if token != "" {
			req.Header.Set("X-Blister-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if token == "secret" && resp.Header.Get("Access-Control-Allow-Origin") != "chrome-extension://abc" {
			t.Fatal("missing CORS header for extension origin")
		}
		return resp.StatusCode
	}

	if c := post("", Capture{URL: "https://x/y.zip"}); c != http.StatusUnauthorized {
		t.Fatalf("no token: %d", c)
	}
	if c := post("wrong", Capture{URL: "https://x/y.zip"}); c != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", c)
	}
	if c := post("secret", Capture{}); c != http.StatusBadRequest {
		t.Fatalf("empty: %d", c)
	}
	if c := post("secret", Capture{URL: "https://x/y.zip", Referer: "https://x"}); c != http.StatusOK || got.Referer != "https://x" {
		t.Fatalf("add: %d %+v", c, got)
	}

	resp, err := http.Get(base + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&p)
	resp.Body.Close()
	if p["app"] != "blister" || p["authorized"] != false {
		t.Fatalf("ping: %v", p)
	}
}
