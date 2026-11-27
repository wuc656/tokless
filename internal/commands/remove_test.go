package commands

import (
	"encoding/json"
	"os"
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
	t.Setenv("TOKLESS_TEST", "1")
	t.Setenv("PATH", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })

	ops := defaultRemoveOps()
	// None of the files or daemons exist; must succeed with 0
	exitCode := runRemoveWith(ops, InitOptions{Yes: true})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestIsToklessInstallationDirRejectsSubdirsAndForeignFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tokless")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tokless.exe", "toklessw.exe", "install.json", "tokless.exe.old", "tokless.exe.1.bak", "tokless.exe.download-123"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !isToklessInstallationDir(dir) {
		t.Fatal("expected pure tokless installation dir to pass verification")
	}

	for _, foreign := range []string{"tokless.old-notes.txt", "tokless.settings.bak", "tokless.download-notes"} {
		p := filepath.Join(dir, foreign)
		if err := os.WriteFile(p, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if isToklessInstallationDir(dir) {
			t.Fatalf("installation dir with user file %q must be rejected", foreign)
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}

	backupSubdir := filepath.Join(dir, "tokless-backups")
	if err := os.MkdirAll(backupSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if isToklessInstallationDir(dir) {
		t.Fatal("installation dir with tokless-backups subdirectory must be rejected")
	}
}

func TestRemoveToklessBinariesInSharedDirPreservesSiblings(t *testing.T) {
	sharedDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tokless", "toklessw", "other-tool"} {
		if err := os.WriteFile(filepath.Join(sharedDir, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	backupDir := filepath.Join(sharedDir, "tokless-backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := removeToklessBinariesInDir(sharedDir); err != nil {
		t.Fatalf("removeToklessBinariesInDir: %v", err)
	}
	if util.Exists(filepath.Join(sharedDir, "tokless")) || util.Exists(filepath.Join(sharedDir, "toklessw")) {
		t.Fatal("tokless binaries were not removed from shared dir")
	}
	if !util.Exists(filepath.Join(sharedDir, "other-tool")) {
		t.Fatal("sibling binary in shared dir was removed")
	}
	if !util.Exists(backupDir) {
		t.Fatal("tokless-backups subdirectory was removed")
	}
}

func TestRunRemoveReturnsNonZeroOnFailure(t *testing.T) {
	t.Setenv("TOKLESS_TEST", "1")
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Cleanup(func() { util.SetHomeOverride("") })

	ops := defaultRemoveOps()
	ops.stopProxy = func() error { return os.ErrPermission }

	if code := runRemoveWith(ops, InitOptions{Yes: true}); code == 0 {
		t.Fatalf("exit code = %d, want non-zero when removal fails without --force", code)
	}
	if code := runRemoveWith(ops, InitOptions{Yes: true, Force: true}); code != 0 {
		t.Fatalf("exit code = %d, want 0 with --force", code)
	}
}

func TestDeepSweepPreservesCommentsAndRemoteMcpIdempotent(t *testing.T) {
	t.Setenv("TOKLESS_TEST", "1")
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Cleanup(func() { util.SetHomeOverride("") })

	cfgPath := util.OpenCodePathsResolved().Config
	initial := "{\n  // OpenCode user config\n  \"theme\": \"dark\",\n  \"mcp\": {\n    // user remote service must stay\n    \"codegraph\": {\n      \"type\": \"remote\",\n      \"url\": \"https://example.com/mcp\"\n    },\n    \"context-mode\": {\n      \"type\": \"local\",\n      \"command\": [\"tokless\", \"run-mcp\", \"context-mode\"]\n    }\n  }\n}\n"
	if err := util.WriteFile(cfgPath, initial); err != nil {
		t.Fatal(err)
	}

	deepSweepAgentConfigs()

	afterFirst, ok := util.ReadFileSafe(cfgPath)
	if !ok {
		t.Fatal("opencode config missing after deepSweepAgentConfigs")
	}
	if !strings.Contains(afterFirst, "// OpenCode user config") || !strings.Contains(afterFirst, "// user remote service must stay") {
		t.Fatalf("comments were stripped:\n%s", afterFirst)
	}
	if !strings.Contains(afterFirst, `"codegraph"`) || !strings.Contains(afterFirst, "https://example.com/mcp") {
		t.Fatalf("user remote codegraph MCP was removed:\n%s", afterFirst)
	}
	if strings.Contains(afterFirst, `"context-mode"`) {
		t.Fatalf("managed context-mode MCP survived:\n%s", afterFirst)
	}

	deepSweepAgentConfigs()

	afterSecond, ok := util.ReadFileSafe(cfgPath)
	if !ok {
		t.Fatal("opencode config missing after second deepSweepAgentConfigs")
	}
	if afterSecond != afterFirst {
		t.Fatalf("second deepSweepAgentConfigs not byte-identical:\n--- first ---\n%s\n--- second ---\n%s", afterFirst, afterSecond)
	}
}

func TestRunRemoveDoesNotCleanUserPathForSharedDir(t *testing.T) {
	t.Setenv("TOKLESS_TEST", "1")
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Cleanup(func() { util.SetHomeOverride("") })

	sharedDir := filepath.Join(t.TempDir(), "Tools")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tokless.exe", "other.exe"} {
		if err := os.WriteFile(filepath.Join(sharedDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cleanCalls := 0
	ops := defaultRemoveOps()
	ops.resolveInstallDir = func() string { return sharedDir }
	ops.cleanUserPath = func(string) error {
		cleanCalls++
		return nil
	}

	if code := runRemoveWith(ops, InitOptions{Yes: true}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if cleanCalls != 0 {
		t.Fatalf("cleanUserPath called %d times for shared dir, want 0", cleanCalls)
	}
}

func TestRunRemovePreservesCommentsCustomLocalMcpAndSiblingHooks(t *testing.T) {
	t.Setenv("TOKLESS_TEST", "1")
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("PATH", t.TempDir())
	t.Cleanup(func() { util.SetHomeOverride("") })

	wsDir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wsDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	spawn := util.WrapAutoIndex("opencode", util.PickMcpSpawn("codegraph", "serve", "--mcp"))
	managedCmdJSON, err := json.Marshal(append([]string{spawn.Command}, spawn.Args...))
	if err != nil {
		t.Fatal(err)
	}

	openCodePath := util.OpenCodePathsResolved().Config
	openCodeInitial := "{\n  // OpenCode user config\n  \"theme\": \"dark\",\n  \"mcp\": {\n    \"codegraph\": {\n      \"type\": \"local\",\n      \"command\": " + string(managedCmdJSON) + ",\n      \"enabled\": true\n    },\n    // custom user local codegraph MCP\n    \"my-index\": {\n      \"type\": \"local\",\n      \"command\": [\"codegraph\", \"serve\", \"--mcp\"]\n    },\n    // custom user remote MCP\n    \"context-mode\": {\n      \"type\": \"remote\",\n      \"url\": \"https://example.com/mcp\"\n    }\n  }\n}\n"
	openCodeWant := "{\n  // OpenCode user config\n  \"theme\": \"dark\",\n  \"mcp\": {\n    // custom user local codegraph MCP\n    \"my-index\": {\n      \"type\": \"local\",\n      \"command\": [\"codegraph\", \"serve\", \"--mcp\"]\n    },\n    // custom user remote MCP\n    \"context-mode\": {\n      \"type\": \"remote\",\n      \"url\": \"https://example.com/mcp\"\n    }\n  }\n}\n"
	if err := util.WriteFile(openCodePath, openCodeInitial); err != nil {
		t.Fatal(err)
	}

	codexHooksPath := agents.CodexRtkHooksFile()
	codexHooksInitial := "{\n  // Codex hooks config\n  \"hooks\": {\n    // PreToolUse keeps sibling user hooks\n    \"PreToolUse\": [\n      {\n        \"matcher\": \"Bash|apply_patch|ctx_.*|codegraph_.*\",\n        \"hooks\": [\n          { \"type\": \"command\", \"command\": \"echo user\" }, // user sibling hook\n          { \"type\": \"command\", \"command\": \"tokless rtk-hook codex\" },\n          { \"type\": \"command\", \"command\": \"context-mode hook codex pretooluse\" }\n        ]\n      }\n    ]\n  }\n}\n"
	codexHooksWant := "{\n  // Codex hooks config\n  \"hooks\": {\n    // PreToolUse keeps sibling user hooks\n    \"PreToolUse\": [\n      {\n        \"matcher\": \"Bash|apply_patch|ctx_.*|codegraph_.*\",\n        \"hooks\": [\n          { \"type\": \"command\", \"command\": \"echo user\" } // user sibling hook\n        ]\n      }\n    ]\n  }\n}\n"
	if err := util.WriteFile(codexHooksPath, codexHooksInitial); err != nil {
		t.Fatal(err)
	}

	wsCodexCfgPath := filepath.Join(wsDir, ".codex", "config.toml")
	wsCodexCfgInitial := "# workspace codex config\n[mcp_servers.context-mode]\ncommand = \"/custom/bin/context-mode\"\nargs = [\"--custom\"]\n\n[mcp_servers.context_mode]\ncommand = \"tokless\"\nargs = [\n  'run-mcp', # single-quoted TOML string + mixed double-quoted\n  \"--tool\",\n  \"context-mode\",\n]\nenabled = true\n"
	wsCodexCfgWant := "# workspace codex config\n[mcp_servers.context-mode]\ncommand = \"/custom/bin/context-mode\"\nargs = [\"--custom\"]\n\n"
	if err := util.WriteFile(wsCodexCfgPath, wsCodexCfgInitial); err != nil {
		t.Fatal(err)
	}

	globalCodexCfgPath := util.CodexPathsResolved().Config
	globalCodexCfgInitial := "[mcp_servers.myserver]\ncommand = \"myserver\"\n\n[mcp_servers.codegraph]\ncommand = \"tokless\"\nargs = [\"run-mcp\", \"--tool\", \"codegraph\"]\n"
	globalCodexCfgWant := "[mcp_servers.myserver]\ncommand = \"myserver\"\n\n"
	if err := util.WriteFile(globalCodexCfgPath, globalCodexCfgInitial); err != nil {
		t.Fatal(err)
	}

	ampCfgPath := util.AmpPathsResolved().Config
	ampCfgInitial := "{\n  // Amp user config\n  \"amp.mcpServers\": {\n    \"tokless-alias\": {\n      \"command\": \"tokless\",\n      \"args\": [\"run-mcp\", \"codegraph\"]\n    },\n    // user amp MCP\n    \"my-tool\": {\n      \"command\": \"my-tool\"\n    }\n  }\n}\n"
	ampCfgWant := "{\n  // Amp user config\n  \"amp.mcpServers\": {\n    // user amp MCP\n    \"my-tool\": {\n      \"command\": \"my-tool\"\n    }\n  }\n}\n"
	if err := util.WriteFile(ampCfgPath, ampCfgInitial); err != nil {
		t.Fatal(err)
	}

	ops := defaultRemoveOps()
	if code := runRemoveWith(ops, InitOptions{Yes: true}); code != 0 {
		t.Fatalf("first runRemoveWith exit code = %d, want 0", code)
	}

	gotOpenCode, ok := util.ReadFileSafe(openCodePath)
	if !ok {
		t.Fatal("opencode config missing after remove")
	}
	if gotOpenCode != openCodeWant {
		t.Fatalf("opencode config mismatch after first remove:\n--- got ---\n%s\n--- want ---\n%s", gotOpenCode, openCodeWant)
	}

	gotCodexHooks, ok := util.ReadFileSafe(codexHooksPath)
	if !ok {
		t.Fatal("codex hooks config missing after remove")
	}
	if gotCodexHooks != codexHooksWant {
		t.Fatalf("codex hooks mismatch after first remove:\n--- got ---\n%s\n--- want ---\n%s", gotCodexHooks, codexHooksWant)
	}

	gotWsCfg, ok := util.ReadFileSafe(wsCodexCfgPath)
	if !ok {
		t.Fatal("workspace codex config missing after remove")
	}
	if gotWsCfg != wsCodexCfgWant {
		t.Fatalf("workspace codex config mismatch after first remove:\n--- got ---\n%s\n--- want ---\n%s", gotWsCfg, wsCodexCfgWant)
	}

	gotGlobalCodex, ok := util.ReadFileSafe(globalCodexCfgPath)
	if !ok {
		t.Fatal("global codex config missing after remove")
	}
	if gotGlobalCodex != globalCodexCfgWant {
		t.Fatalf("global codex config mismatch after first remove:\n--- got ---\n%s\n--- want ---\n%s", gotGlobalCodex, globalCodexCfgWant)
	}

	gotAmpCfg, ok := util.ReadFileSafe(ampCfgPath)
	if !ok {
		t.Fatal("amp config missing after remove")
	}
	if gotAmpCfg != ampCfgWant {
		t.Fatalf("amp config mismatch after first remove:\n--- got ---\n%s\n--- want ---\n%s", gotAmpCfg, ampCfgWant)
	}

	if code := runRemoveWith(ops, InitOptions{Yes: true}); code != 0 {
		t.Fatalf("second runRemoveWith exit code = %d, want 0", code)
	}
	gotOpenCode2, _ := util.ReadFileSafe(openCodePath)
	if gotOpenCode2 != gotOpenCode {
		t.Fatalf("opencode config changed on second remove:\n--- first ---\n%s\n--- second ---\n%s", gotOpenCode, gotOpenCode2)
	}
	gotCodexHooks2, _ := util.ReadFileSafe(codexHooksPath)
	if gotCodexHooks2 != gotCodexHooks {
		t.Fatalf("codex hooks changed on second remove:\n--- first ---\n%s\n--- second ---\n%s", gotCodexHooks, gotCodexHooks2)
	}
	gotWsCfg2, _ := util.ReadFileSafe(wsCodexCfgPath)
	if gotWsCfg2 != gotWsCfg {
		t.Fatalf("workspace codex config changed on second remove:\n--- first ---\n%s\n--- second ---\n%s", gotWsCfg, gotWsCfg2)
	}
	gotGlobalCodex2, _ := util.ReadFileSafe(globalCodexCfgPath)
	if gotGlobalCodex2 != gotGlobalCodex {
		t.Fatalf("global codex config changed on second remove:\n--- first ---\n%s\n--- second ---\n%s", gotGlobalCodex, gotGlobalCodex2)
	}
	gotAmpCfg2, _ := util.ReadFileSafe(ampCfgPath)
	if gotAmpCfg2 != gotAmpCfg {
		t.Fatalf("amp config changed on second remove:\n--- first ---\n%s\n--- second ---\n%s", gotAmpCfg, gotAmpCfg2)
	}
}
