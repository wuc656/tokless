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
	_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	_ = c.Process.Kill()
}

func PrepareSysProcAttr(_ *exec.Cmd, _ bool) {}

func prepareSysProcAttr(_ *exec.Cmd, _ bool) {}
