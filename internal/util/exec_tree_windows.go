//go:build windows

package util

import (
	"os/exec"
	"strconv"
	"syscall"
)

func prepareCommandForTreeKill(_ *exec.Cmd) {}

func killProcessTree(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	tk := exec.Command("taskkill", "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F")
	tk.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = tk.Run()
	_ = c.Process.Kill()
}

func PrepareSysProcAttr(c *exec.Cmd, hide bool) {
	if hide {
		if c.SysProcAttr == nil {
			c.SysProcAttr = &syscall.SysProcAttr{}
		}
		c.SysProcAttr.CreationFlags |= 0x08000000
		c.SysProcAttr.HideWindow = true
	}
}

func prepareSysProcAttr(c *exec.Cmd, hide bool) {
	PrepareSysProcAttr(c, hide)
}
