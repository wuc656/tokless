//go:build windows

package main

import (
	"os"
	"syscall"
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modUser32   = syscall.NewLazyDLL("user32.dll")

	procGetConsoleWindow = modKernel32.NewProc("GetConsoleWindow")
	procFreeConsole      = modKernel32.NewProc("FreeConsole")
	procShowWindow       = modUser32.NewProc("ShowWindow")
)

func init() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "__proxy-watch", "__proxy-ensure", "__byok-gateway-serve", "__proxy-run":
			hideMyConsole()
		}
	}
}

func hideMyConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		// SW_HIDE = 0
		procShowWindow.Call(hwnd, 0)
		procFreeConsole.Call()
	}
}
