package app

import (
	"os"
	"path/filepath"
	"strings"
)

// ExistingDirectory gives the native picker a valid starting location without
// creating folders simply because someone opened a dialog.
func existingDirectory(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		for {
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				return path
			}
			if err != nil && !os.IsNotExist(err) {
				break
			}
			parent := filepath.Dir(path)
			if parent == path {
				break
			}
			path = parent
		}
	}
	return ""
}
