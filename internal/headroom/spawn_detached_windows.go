//go:build windows

package headroom

import (
	"os/exec"
	"syscall"
)

// spawnDetached starts cmd independently without showing a console window.
func spawnDetached(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000
	cmd.SysProcAttr.HideWindow = true
	return cmd.Start()
}
