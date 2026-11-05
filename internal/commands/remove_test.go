package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/agents"
	toolsPkg "github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

func TestRunRemoveDryRun(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })

	calledStopProxy := false
	calledDisableAutostart := false
	calledRemoveAll := false

	ops := removeOps{
		stopProxy: func() error {
			calledStopProxy = true
			return nil
		},
		stopBYOK:         func() error { return nil },
		stopCopilotProxy: func() error { return nil },
		stopGrokProxy:    func() error { return nil },
		disableAutostart: func() error {
			calledDisableAutostart = true
			return nil
		},
		removeAll: func(string) error {
			calledRemoveAll = true
			return nil
		},
		removeFile:    func(string) error { return nil },
		cleanUserPath: func(string) error { return nil },
		uninstallNpm:  func(...string) error { return nil },
		uninstallRtk:  func(string) error { return nil },
		selfDelete:    func(string) error { return nil },
	}

	exitCode := runRemoveWith(ops, InitOptions{DryRun: true})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if calledStopProxy || calledDisableAutostart || calledRemoveAll {
		t.Fatal("dry-run should not execute removal operations")
	}
}

func TestRunRemoveComplete(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Cleanup(func() { util.SetHomeOverride("") })

	// Pre-seed Claude with codegraph and legacy projectmem
	claudePath := util.ClaudeCodePaths().GlobalJSON
	_ = util.EnsureDir(filepath.Dir(claudePath))
	_ = util.WriteFile(claudePath, `{"mcpServers":{"codegraph":{"command":"tokless","args":["run-mcp","--tool","codegraph"]},"projectmem":{"command":"tokless"}}}`)

	// Pre-seed Codex with RTK hook and TOML blocks
	codexHooks := agents.CodexRtkHooksFile()
	_ = util.EnsureDir(filepath.Dir(codexHooks))
	_ = util.WriteFile(codexHooks, `{"hooks":{"PreToolUse":[{"type":"command","command":"tokless rtk-hook codex"}]}}`)
	codexCfg := util.CodexPathsResolved().Config
	_ = util.WriteFile(codexCfg, "[mcp_servers.codegraph]\ncommand = \"tokless\"\n\n[mcp_servers.projectmem]\ncommand = \"tokless\"\n")

	// Pre-seed Instruction file
	claudeInst := toolsPkg.InstructionPath("claude")
	_ = util.EnsureDir(filepath.Dir(claudeInst))
	_ = util.WriteFile(claudeInst, "# Agent Instructions\n\n## Code Index (codegraph)\nDocs\n")

	// Pre-seed dummy RTK binary
	rtkDir := t.TempDir()
	rtkBin := filepath.Join(rtkDir, "rtk")
	if util.IsWin {
		rtkBin += ".exe"
	}
	_ = util.WriteFile(rtkBin, "dummy-rtk")
	t.Setenv("PATH", rtkDir+string(filepath.ListSeparator)+t.TempDir())

	calledStopProxy := false
	calledStopBYOK := false
	calledDisableAutostart := false
	calledUninstallNpm := false
	calledUninstallRtk := false
	_ = "cleanedPathDir"
	_ = "selfDeletedDir"
	var removedDirs []string

	ops := removeOps{
		stopProxy: func() error {
			calledStopProxy = true
			return nil
		},
		stopBYOK: func() error {
			calledStopBYOK = true
			return nil
		},
		stopCopilotProxy: func() error { return nil },
		stopGrokProxy:    func() error { return nil },
		disableAutostart: func() error {
			calledDisableAutostart = true
			return nil
		},
		removeAll: func(path string) error {
			removedDirs = append(removedDirs, path)
			return nil
		},
		removeFile: func(string) error { return nil },
		cleanUserPath: func(dir string) error {
			_ = dir
			return nil
		},
		uninstallNpm: func(pkgs ...string) error {
			calledUninstallNpm = true
			return nil
		},
		uninstallRtk: func(p string) error {
			calledUninstallRtk = true
			return nil
		},
		selfDelete: func(dir string) error {
			_ = dir
			return nil
		},
	}

	exitCode := runRemoveWith(ops, InitOptions{Yes: true})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if !calledStopProxy {
		t.Error("stopProxy was not called")
	}
	if !calledStopBYOK {
		t.Error("stopBYOK was not called")
	}
	if !calledDisableAutostart {
		t.Error("disableAutostart was not called")
	}
	if !calledUninstallNpm {
		t.Error("uninstallNpm was not called")
	}
	if !calledUninstallRtk {
		t.Error("uninstallRtk was not called")
	}

	// Verify Claude configs purged
	raw, _ := util.ReadFileSafe(claudePath)
	if strings.Contains(raw, "tokless") || strings.Contains(raw, "projectmem") || strings.Contains(raw, "codegraph") {
		t.Fatalf("Claude config still contains Tokless entries: %s", raw)
	}

	// Verify Codex configs purged
	raw, _ = util.ReadFileSafe(codexHooks)
	if strings.Contains(raw, "rtk-hook") || strings.Contains(raw, "tokless") {
		t.Fatalf("Codex hooks still contains Tokless hook: %s", raw)
	}
	raw, _ = util.ReadFileSafe(codexCfg)
	if strings.Contains(raw, "mcp_servers.codegraph") || strings.Contains(raw, "mcp_servers.projectmem") {
		t.Fatalf("Codex config.toml still contains Tokless blocks: %s", raw)
	}

	// Verify instructions removed
	if util.Exists(claudeInst) {
		raw, _ = util.ReadFileSafe(claudeInst)
		if strings.TrimSpace(raw) != "" {
			t.Fatalf("Claude instruction file not purged: %s", raw)
		}
	}
}

func TestRunRemoveWhenAlreadyUninstalled(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })

	ops := defaultRemoveOps()
	// None of the files or daemons exist; must succeed with 0
	exitCode := runRemoveWith(ops, InitOptions{Yes: true})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}
