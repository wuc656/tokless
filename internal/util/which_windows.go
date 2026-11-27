package util

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// A single directory query rules out every PATHEXT candidate on a miss.
func missingExecutablePrefix(dir, bin string, exts []string) bool {
	if !filepath.IsLocal(bin) || strings.ContainsAny(bin, `/\:*?"<>|`) {
		return false
	}
	for _, ext := range exts {
		if strings.ContainsAny(ext, `/\:`) {
			return false
		}
	}
	search := filepath.Join(dir, bin+"*")
	if len(search) >= syscall.MAX_PATH || !filepath.IsAbs(search) {
		return false
	}
	pattern, err := syscall.UTF16PtrFromString(search)
	if err != nil {
		return false
	}
	var data syscall.Win32finddata
	handle, err := syscall.FindFirstFile(pattern, &data)
	if err == nil {
		syscall.FindClose(handle)
		return false
	}
	if err != syscall.ERROR_FILE_NOT_FOUND && err != syscall.ERROR_PATH_NOT_FOUND {
		return false
	}
	// Track the directory in Go's test cache so a newly installed binary
	// invalidates a cached miss, without logging every absent suffix.
	_, _ = os.Stat(dir)
	return true
}
