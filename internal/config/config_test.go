package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDefaultsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := st.Get()
	if s.Connections != 16 || s.MaxActive != 4 {
		t.Fatalf("unexpected defaults: %+v", s)
	}
	s.Connections = 999
	s.SpeedLimit = 1 << 20
	if _, err := st.Set(s); err != nil {
		t.Fatal(err)
	}
	st2, _ := Open(dir)
	got := st2.Get()
	if got.Connections != 64 {
		t.Fatalf("connections not clamped: %d", got.Connections)
	}
	if got.SpeedLimit != 1<<20 {
		t.Fatalf("speed limit lost: %d", got.SpeedLimit)
	}
}

func TestCorruptFileFallsBack(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(p, []byte("{nope"), 0o644)
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Get().Connections != 16 {
		t.Fatal("expected defaults")
	}
	if _, err := os.Stat(p + ".bad"); err != nil {
		t.Fatal("bad file not preserved")
	}
}
