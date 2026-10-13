package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func joinPath(dir, name string) string {
	if name == "" {
		return dir
	}
	return filepath.Join(dir, name)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// uniqueName returns name, or "name (2).ext" etc., so it collides with
// neither an existing file nor any name in taken.
func uniqueName(dir, name string, taken func(string) bool) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	// keep ".tar.gz" together
	if strings.HasSuffix(strings.ToLower(base), ".tar") {
		ext = base[len(base)-4:] + ext
		base = base[:len(base)-4]
	}
	cand := name
	for i := 2; ; i++ {
		if !exists(filepath.Join(dir, cand)) && !exists(filepath.Join(dir, cand+".part")) && (taken == nil || !taken(cand)) {
			return cand
		}
		cand = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// moveFile moves src without ever overwriting an existing destination.
func moveFile(src, dst string) error {
	// Linking is atomic, cheap for large downloads, and fails if dst exists.
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	in.Close()
	return os.Remove(src)
}
