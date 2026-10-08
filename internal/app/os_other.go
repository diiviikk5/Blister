//go:build !windows

package app

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func openPath(p string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", p).Start()
	}
	return exec.Command("xdg-open", p).Start()
}

func revealPath(p string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", p).Start()
	}
	return exec.Command("xdg-open", filepath.Dir(p)).Start()
}

func setStartOnBoot(bool) error { return nil }

func runShell(cmdline, dir string) error {
	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Dir = dir
	return cmd.Start()
}
