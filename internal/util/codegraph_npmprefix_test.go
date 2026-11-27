package util

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func fakeBinIn(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho 1.2.3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// skipIfRealCodegraphNearNode skips when a real codegraph sits where the node-dir fallback looks.
func skipIfRealCodegraphNearNode(t *testing.T) {
	t.Helper()
	if n := ResolveNodeBinary(); n != "" && Exists(filepath.Join(filepath.Dir(n), "codegraph")) {
		t.Skip("real codegraph next to node")
	}
}

// A codegraph under a custom npm prefix that is not on PATH must still resolve (issue #69).
func TestResolveCodegraphBinCustomNpmPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell shim")
	}
	home := t.TempDir()
	SetHomeOverride(home)
	defer SetHomeOverride("")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("npm_config_prefix", "")
	t.Setenv("NPM_CONFIG_PREFIX", "")
	skipIfRealCodegraphNearNode(t)

	prefix := filepath.Join(home, ".npm-global")
	want := fakeBinIn(t, filepath.Join(prefix, "bin"), "codegraph")

	orig := npmPrefix
	npmPrefix = func() string { return prefix }
	defer func() { npmPrefix = orig }()

	if got := ResolveCodegraphBin(); got != want {
		t.Fatalf("custom npm prefix: want %q, got %q", want, got)
	}
}

// npm_config_prefix resolves even when `npm config get prefix` yields nothing.
func TestResolveCodegraphBinEnvNpmPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell shim")
	}
	SetHomeOverride(t.TempDir())
	defer SetHomeOverride("")
	t.Setenv("PATH", t.TempDir())
	skipIfRealCodegraphNearNode(t)
	prefix := t.TempDir()
	t.Setenv("npm_config_prefix", prefix)
	want := fakeBinIn(t, filepath.Join(prefix, "bin"), "codegraph")

	orig := npmPrefix
	npmPrefix = func() string { return "" }
	defer func() { npmPrefix = orig }()

	if got := ResolveCodegraphBin(); got != want {
		t.Fatalf("env npm prefix: want %q, got %q", want, got)
	}
}

// context-mode under a custom npm prefix must spawn by absolute path, not npx (issue #69).
func TestPickMcpSpawnContextModeCustomNpmPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell shim")
	}
	SetHomeOverride(t.TempDir())
	defer SetHomeOverride("")
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	want := fakeBinIn(t, filepath.Join(prefix, "bin"), "context-mode")

	orig := npmPrefix
	npmPrefix = func() string { return prefix }
	defer func() { npmPrefix = orig }()

	spawn := PickMcpSpawn("context-mode")
	if !slices.Contains(spawn.Args, want) || slices.Contains(spawn.Args, "--no-install") {
		t.Fatalf("want absolute %q and no npx fallback, got %v", want, spawn.Args)
	}
}

func fakeCodegraphCmdIn(t *testing.T, dir, version string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if IsWin {
		p := filepath.Join(dir, "codegraph.cmd")
		body := "@echo off\r\necho " + version + "\r\n"
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := filepath.Join(dir, "codegraph")
	body := "#!/bin/sh\necho " + version + "\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveCodegraphBinPrefersNpmPrefixOverPath(t *testing.T) {
	home := t.TempDir()
	SetHomeOverride(home)
	defer SetHomeOverride("")

	pathDir := t.TempDir()
	pathBin := fakeCodegraphCmdIn(t, pathDir, "1.6.2")
	pathVal := pathDir
	if IsWin {
		if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
			pathVal += ";" + filepath.Join(sysRoot, "System32")
		}
	}
	t.Setenv("PATH", pathVal)
	t.Setenv("npm_config_prefix", "")
	t.Setenv("NPM_CONFIG_PREFIX", "")

	prefix := t.TempDir()
	want := fakeCodegraphCmdIn(t, npmGlobalBinDir(prefix, IsWin), "1.6.4")

	orig := npmPrefix
	npmPrefix = func() string { return prefix }
	defer func() { npmPrefix = orig }()

	got := ResolveCodegraphBin()
	if !sameExecutablePath(got, want) {
		t.Fatalf("ResolveCodegraphBin() = %q, want npmPrefix bin %q (over PATH %q)", got, want, pathBin)
	}

	// Restore PATH to simulate initial process PATH before PrependProcessPath mutated it.
	t.Setenv("PATH", pathVal)
	shadowed, active := CodegraphPathConflict()
	if !sameExecutablePath(shadowed, pathBin) || !sameExecutablePath(active, want) {
		t.Fatalf("CodegraphPathConflict() = (%q, %q), want (%q, %q)", shadowed, active, pathBin, want)
	}
}

func TestWhichNpmGlobalPrefersNpmPrefixOverPath(t *testing.T) {
	home := t.TempDir()
	SetHomeOverride(home)
	defer SetHomeOverride("")

	pathDir := t.TempDir()
	ctxName := "context-mode"
	if IsWin {
		ctxName = "context-mode.cmd"
	}
	pathBin := fakeBinIn(t, pathDir, ctxName)
	t.Setenv("PATH", pathDir)
	t.Setenv("npm_config_prefix", "")
	t.Setenv("NPM_CONFIG_PREFIX", "")

	prefix := t.TempDir()
	want := fakeBinIn(t, npmGlobalBinDir(prefix, IsWin), ctxName)

	orig := npmPrefix
	npmPrefix = func() string { return prefix }
	defer func() { npmPrefix = orig }()

	got := whichNpmGlobal("context-mode")
	if !sameExecutablePath(got, want) {
		t.Fatalf("whichNpmGlobal() = %q, want npmPrefix bin %q (over PATH %q)", got, want, pathBin)
	}
}
