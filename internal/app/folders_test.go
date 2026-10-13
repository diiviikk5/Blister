package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickerStartsAtNearestExistingDirectory(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "Downloads", "Blister", "Archives")
	if got := existingDirectory(missing); got != root {
		t.Fatalf("start = %q, want %q", got, root)
	}
	if _, err := os.Stat(filepath.Join(root, "Downloads")); !os.IsNotExist(err) {
		t.Fatal("opening picker created folders")
	}
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := existingDirectory(file); got != root {
		t.Fatalf("file start = %q", got)
	}
	if got := existingDirectory("", "  ", root); got != root {
		t.Fatalf("fallback = %q", got)
	}
}
