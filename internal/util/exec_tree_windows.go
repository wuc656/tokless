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
	KillProcessTreePID(c.Process.Pid)
	_ = c.Process.Kill()
}

// KillProcessTreePID terminates a process and all its child processes on Windows.
func KillProcessTreePID(pid int) {
	if pid <= 0 {
		return
	}
	tk := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	tk.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = tk.Run()
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
