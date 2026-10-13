package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	os.WriteFile(src, []byte("download"), 0600)
	os.WriteFile(dst, []byte("existing"), 0600)
	if err := moveFile(src, dst); err == nil {
		t.Fatal("overwrote existing file")
	}
	for p, want := range map[string]string{src: "download", dst: "existing"} {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != want {
			t.Fatalf("lost %s: %q, %v", p, got, err)
		}
	}
}

func TestRenameKeepsPartialAndChosenName(t *testing.T) {
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	task, err := m.Add(AddRequest{URL: "https://example.com/original.bin", Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("partial bytes")
	if err := os.WriteFile(task.PartPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(task.ID, "chosen.bin"); err != nil {
		t.Fatal(err)
	}
	renamed, _ := m.Get(task.ID)
	got, err := os.ReadFile(renamed.PartPath())
	if err != nil || !bytes.Equal(got, data) || !renamed.NameFixed {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	other, err := m.Add(AddRequest{URL: "https://example.com/other.bin", Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(task.ID, other.Name); err == nil {
		t.Fatal("claimed another task's name")
	}
	still, _ := m.Get(task.ID)
	if still.Name != renamed.Name {
		t.Fatal("failed rename changed task state")
	}
}

func TestRejectMalformedChecksumsBeforeDownloading(t *testing.T) {
	m, _ := newManager(t, t.TempDir(), t.TempDir())
	defer m.Close()
	for _, digest := range []string{"sha256:00", "md5:", "sha1:" + strings.Repeat("z", 40)} {
		if _, err := m.Add(AddRequest{URL: "https://example.com/file", Checksum: digest}); err == nil {
			t.Errorf("accepted %q", digest)
		}
	}
	if len(m.List()) != 0 {
		t.Fatal("invalid downloads queued")
	}
}

func TestGuessLocalTorrentName(t *testing.T) {
	for _, raw := range []string{`C:\Downloads\pack.torrent`, "file:///C:/Downloads/pack.torrent", "/tmp/pack.torrent"} {
		if got := GuessName(raw, KindTorrent); got != "pack.torrent" {
			t.Errorf("GuessName(%q) = %q", raw, got)
		}
	}
}
