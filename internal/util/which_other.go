//go:build !windows

package util

func missingExecutablePrefix(dir, bin string, exts []string) bool { return false }
