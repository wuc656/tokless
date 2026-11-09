package util

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNodeLTSVersion(t *testing.T) {
	entries := []nodeDistEntry{
		{
			Version: "v23.0.0",
			Files:   []string{"win-x64-zip", "win-arm64-zip"},
			LTS:     json.RawMessage("false"),
		},
		{
			Version: "v22.1.0",
			Files:   []string{"win-x64-zip"},
			LTS:     json.RawMessage(`"Jod"`),
		},
		{
			Version: "v20.0.0",
			Files:   []string{"win-x64-zip", "win-arm64-zip"},
			LTS:     json.RawMessage(`"Iron"`),
		},
		{
			Version: "v18.0.0",
			Files:   []string{"win-x64-zip"},
			LTS:     json.RawMessage("null"),
		},
	}

	tests := []struct {
		name     string
		arch     string
		expected string
		ok       bool
	}{
		{
			name:     "x64 gets newest LTS",
			arch:     "x64",
			expected: "v22.1.0",
			ok:       true,
		},
		{
			name:     "arm64 falls back to older LTS",
			arch:     "arm64",
			expected: "v20.0.0",
			ok:       true,
		},
		{
			name:     "unsupported arch",
			arch:     "x86",
			expected: "",
			ok:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nodeLTSVersion(entries, "win-"+tc.arch+"-zip")
			if ok != tc.ok {
				t.Errorf("nodeLTSVersion() ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.expected {
				t.Errorf("nodeLTSVersion() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestExtractZipStripRoot(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "dest")

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	files := []struct {
		name    string
		content string
	}{
		{"node-v22.0.0-win-x64/node.exe", "x"},
		{"node-v22.0.0-win-x64/node_modules/npm/bin/npm.cmd", "y"},
		{"node-v22.0.0-win-x64/", ""},
		{"node-v22.0.0-win-x64/../evil.txt", "evil"},
	}

	for _, f := range files {
		fw, err := zw.CreateHeader(&zip.FileHeader{
			Name:   f.name,
			Method: zip.Deflate,
		})
		if err != nil {
			t.Fatalf("Failed to create zip header for %s: %v", f.name, err)
		}

		if f.content != "" {
			fw.Write([]byte(f.content))
		}
	}
	zw.Close()

	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write zip file: %v", err)
	}

	if err := extractZipStripRoot(zipPath, destDir); err != nil {
		t.Fatalf("extractZipStripRoot failed: %v", err)
	}

	expectedFiles := []struct {
		path    string
		content string
	}{
		{filepath.Join(destDir, "node.exe"), "x"},
		{filepath.Join(destDir, "node_modules", "npm", "bin", "npm.cmd"), "y"},
	}

	for _, ef := range expectedFiles {
		content, err := os.ReadFile(ef.path)
		if err != nil {
			t.Errorf("Expected file %s not found: %v", ef.path, err)
			continue
		}
		if string(content) != ef.content {
			t.Errorf("File %s content = %q, want %q", ef.path, string(content), ef.content)
		}
	}

	evilPath := filepath.Join(tmpDir, "evil.txt")
	if _, err := os.Stat(evilPath); !os.IsNotExist(err) {
		t.Errorf("Evil file was extracted to %s", evilPath)
	}
}

func TestWriteExtractedFileOverwritesLockedExecutable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific locked executable test")
	}
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "dummy.exe")
	// Copy a real Windows executable that runs in background
	timeoutExe := filepath.Join(os.Getenv("WINDIR"), "System32", "timeout.exe")
	if !Exists(timeoutExe) {
		t.Skip("timeout.exe not found")
	}
	srcBytes, err := os.ReadFile(timeoutExe)
	if err != nil {
		t.Skip("cannot read timeout.exe")
	}
	if err := os.WriteFile(exePath, srcBytes, 0o755); err != nil {
		t.Fatal(err)
	}

	// Spawn the executable so it is locked by Windows
	cmd := exec.Command(exePath, "30")
	if err := cmd.Start(); err != nil {
		t.Skip("failed to start timeout.exe:", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(exePath + ".old")
	}()

	// Verify direct OpenFile with O_TRUNC fails with permission error
	_, directErr := os.OpenFile(exePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if directErr == nil {
		t.Fatal("expected permission error when opening running executable directly")
	}

	// Verify writeExtractedFile safely handles the lock via .old rename
	newContent := "new-version-binary"
	if err := writeExtractedFile(exePath, strings.NewReader(newContent), 0o755); err != nil {
		t.Fatalf("writeExtractedFile failed to overwrite running binary: %v", err)
	}

	// Verify new content was written
	readBack, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(readBack) != newContent {
		t.Fatalf("expected %q, got %q", newContent, string(readBack))
	}
}
