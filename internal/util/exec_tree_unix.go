//go:build !windows

package util

import (
	"os/exec"
	"syscall"
)

func prepareCommandForTreeKill(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
}

func killProcessTree(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	KillProcessTreePID(c.Process.Pid)
	_ = c.Process.Kill()
}

// KillProcessTreePID terminates a process and its process group on Unix.
func KillProcessTreePID(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func PrepareSysProcAttr(_ *exec.Cmd, _ bool) {}

func prepareSysProcAttr(_ *exec.Cmd, _ bool) {}
