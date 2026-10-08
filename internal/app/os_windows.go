package app

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func openPath(p string) error {
	return hidden(exec.Command("rundll32", "url.dll,FileProtocolHandler", p)).Start()
}

func revealPath(p string) error {
	// explorer wants the /select, and the path as one argument.
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + p + `"`}
	return cmd.Start()
}

// runShell runs a user hook through cmd without flashing a console.
func runShell(cmdline, dir string) error {
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CmdLine: `cmd /S /C "` + cmdline + `"`}
	cmd.Dir = dir
	return cmd.Start()
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func setStartOnBoot(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		err := k.DeleteValue("Blister")
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue("Blister", `"`+exe+`" --minimized`)
}
