//go:build windows

package headroom

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type processIdentityInfo struct {
	Executable string
	Args       []string
	Start      string
}

type windowsProcess struct {
	Executable string `json:"ExecutablePath"`
	Command    string `json:"CommandLine"`
	Created    string `json:"CreationDate"`
}

func processIdentitySupported() bool { return true }

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	processOpen                = kernel32.NewProc("OpenProcess")
	processClose               = kernel32.NewProc("CloseHandle")
	processExitCode            = kernel32.NewProc("GetExitCodeProcess")
	queryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	getProcessTimes            = kernel32.NewProc("GetProcessTimes")
)

const processQueryLimitedInformation = 0x1000

func getProcessExecutableAndStart(pid int) (string, string, error) {
	handle, _, callErr := processOpen.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		return "", "", callErr
	}
	defer processClose.Call(handle)

	var buf [1024]uint16
	size := uint32(len(buf))
	r1, _, err := queryFullProcessImageNameW.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return "", "", err
	}
	exe := syscall.UTF16ToString(buf[:size])

	var creationTime, exitTime, kernelTime, userTime syscall.Filetime
	r2, _, err := getProcessTimes.Call(
		handle,
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if r2 == 0 {
		return exe, "", err
	}
	t := time.Unix(0, creationTime.Nanoseconds()).UTC()
	start := fmt.Sprintf("/Date(%d)/", t.UnixMilli())
	return exe, start, nil
}

func identifyProcess(pid int) (processIdentityInfo, error) {
	// First, obtain accurate executable path and process creation time via native Win32 API.
	// This works across UAC integrity levels without requiring administrator privileges.
	exe, start, err := getProcessExecutableAndStart(pid)
	if err != nil {
		return processIdentityInfo{}, err
	}
	executable, err := filepath.Abs(exe)
	if err != nil {
		executable = exe
	}

	// Try querying WMI for command line arguments. Note that on Windows, if the target process
	// has higher integrity level than caller, WMI returns null for CommandLine.
	filter := fmt.Sprintf("ProcessId = %d", pid)
	script := "$p = Get-CimInstance Win32_Process -Filter '" + filter + "'; if ($null -eq $p) { exit 1 }; $p | Select-Object ExecutablePath,CommandLine,CreationDate | ConvertTo-Json -Compress"
	var out []byte
	for _, shell := range []string{"powershell", "pwsh"} {
		if _, lookErr := exec.LookPath(shell); lookErr != nil {
			continue
		}
		cmd := exec.Command(shell, "-NoProfile", "-Command", script)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, HideWindow: true}
		out, err = cmd.Output()
		if err == nil {
			break
		}
	}

	var args []string
	if err == nil && len(out) > 0 {
		var p windowsProcess
		if json.Unmarshal(out, &p) == nil && p.Command != "" {
			parsed := windowsCommandLineArgs(p.Command)
			if len(parsed) >= 2 {
				args = parsed[1:]
			}
			if start == "" && p.Created != "" {
				start = p.Created
			}
		}
	}

	return processIdentityInfo{Executable: executable, Args: args, Start: start}, nil
}

func windowsCommandLineArgs(command string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case (c == ' ' || c == '\t') && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func killProcess(proc *os.Process) error { return proc.Kill() }
func processGone(proc *os.Process) bool {
	handle, _, callErr := processOpen.Call(processQueryLimitedInformation, 0, uintptr(proc.Pid))
	if handle == 0 {
		return callErr == syscall.Errno(87)
	}
	defer processClose.Call(handle)
	var code uint32
	ret, _, _ := processExitCode.Call(handle, uintptr(unsafe.Pointer(&code)))
	return ret != 0 && code != 259
}
