package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindBinaryKnownDir(t *testing.T) {
	origIsWin := IsWin
	defer func() { IsWin = origIsWin }()
	IsWin = false

	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	extraDir := t.TempDir()
	claudePath := filepath.Join(extraDir, "claude")
	os.WriteFile(claudePath, []byte("dummy"), 0755)

	res := FindBinary("claude", []string{extraDir})
	if res != claudePath {
		t.Errorf("Expected FindBinary to return %s, got %s", claudePath, res)
	}

	pathEnv := os.Getenv("PATH")
	if !strings.HasPrefix(pathEnv, extraDir) {
		t.Errorf("Expected PATH to start with %s, got %s", extraDir, pathEnv)
	}
}

func TestFindBinaryWindowsExe(t *testing.T) {
	origIsWin := IsWin
	defer func() { IsWin = origIsWin }()
	IsWin = true

	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	// Test .exe
	extraDirExe := t.TempDir()
	claudeExePath := filepath.Join(extraDirExe, "claude.exe")
	os.WriteFile(claudeExePath, []byte("dummy"), 0755)

	resExe := FindBinary("claude", []string{extraDirExe})
	if resExe != claudeExePath {
		t.Errorf("Expected FindBinary to return %s, got %s", claudeExePath, resExe)
	}

	t.Setenv("PATH", emptyDir)
	extraDirCmd := t.TempDir()
	claudeCmdPath := filepath.Join(extraDirCmd, "claude.cmd")
	os.WriteFile(claudeCmdPath, []byte("dummy"), 0755)

	resCmd := FindBinary("claude", []string{extraDirCmd})
	if resCmd != claudeCmdPath {
		t.Errorf("Expected FindBinary to return %s, got %s", claudeCmdPath, resCmd)
	}
}

func TestFindBinaryMiss(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	extraDir := t.TempDir()

	res := FindBinary("claude", []string{extraDir})
	if res != "" {
		t.Errorf("Expected FindBinary to miss and return empty string, got %s", res)
	}
}

func TestWhichWithExtensionOnWindows(t *testing.T) {
	origIsWin := IsWin
	defer func() { IsWin = origIsWin }()
	IsWin = true

	dir := t.TempDir()
	binPath := filepath.Join(dir, "git.exe")
	if err := os.WriteFile(binPath, []byte("dummy"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	// Search with extension
	if got := Which("git.exe"); !EqualPaths(got, binPath) {
		t.Errorf("Which(\"git.exe\") = %q, want %q", got, binPath)
	}

	// Search without extension
	if got := Which("git"); !EqualPaths(got, binPath) {
		t.Errorf("Which(\"git\") = %q, want %q", got, binPath)
	}
}

func TestWhichSeesNewWindowsCandidatesInPATHEXTOrder(t *testing.T) {
	if !IsWin {
		t.Skip("Windows executable lookup")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".CMD;.EXE")
	if got := Which("probe"); got != "" {
		t.Fatalf("unexpected candidate %q", got)
	}
	for _, ext := range []string{".EXE", ".CMD"} {
		want := filepath.Join(dir, "probe"+ext)
		if err := os.WriteFile(want, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := Which("probe"); got != want {
			t.Fatalf("Which returned %q after creating %q", got, want)
		}
	}
}
