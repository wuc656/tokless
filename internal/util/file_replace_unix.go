//go:build !windows

package util

import "os"

// ReplaceFile replaces path with tmpPath atomically and safely across OSes.
func ReplaceFile(tmpPath, path string) error {
	return replaceFile(tmpPath, path)
}

func replaceFile(tmpPath, path string) error {
	return os.Rename(tmpPath, path)
}
