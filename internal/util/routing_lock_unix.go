//go:build !windows

package util

import (
	"fmt"
	"os"
	"syscall"
)

// AcquireFileLock opens or creates path and acquires a non-blocking OS-managed exclusive lock released on unlock or process exit.
func AcquireFileLock(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func acquireRoutingFileLock(path string) (func(), error) {
	unlock, err := AcquireFileLock(path)
	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, fmt.Errorf("routing store is locked")
		}
		return nil, err
	}
	return unlock, nil
}
