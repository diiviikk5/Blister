//go:build !windows

package media

import "os/exec"

func hide(*exec.Cmd) {}

func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
