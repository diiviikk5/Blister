package media

import (
	"os/exec"
	"strconv"
	"syscall"
)

// hide stops a console window flashing up for each child process.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// killTree ends yt-dlp and its children; the Windows build is a launcher
// that spawns the real process, so killing only the parent leaks it.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	k := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	hide(k)
	if err := k.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
