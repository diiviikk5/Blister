package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

// moveFile renames src to dst, falling back to copy across volumes.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) && !isCrossDevice(err) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	in.Close()
	return os.Remove(src)
}

func isCrossDevice(err error) bool {
	var le *os.LinkError
	if errors.As(err, &le) {
		s := le.Err.Error()
		return strings.Contains(s, "cross-device") || strings.Contains(s, "different disk drive")
	}
	return false
}
