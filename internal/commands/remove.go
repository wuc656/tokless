package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/core"
	headroompkg "github.com/HoangP8/tokless/internal/headroom"
	toolsPkg "github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

type removeOps struct {
	stopProxy        func() error
	stopBYOK         func() error
	stopCopilotProxy func() error
	stopGrokProxy    func() error
	disableAutostart func() error
	removeAll        func(string) error
	removeFile       func(string) error
	cleanUserPath    func(string) error
	uninstallNpm     func(pkgs ...string) error
	uninstallRtk     func(string) error
	selfDelete       func(string) error
}

func defaultRemoveOps() removeOps {
	isTest := util.IsRunningInTest()
	return removeOps{
		stopProxy: func() error {
			if isTest {
				return nil
			}
			return headroompkg.StopProxy()
		},
		stopBYOK: func() error {
			if isTest {
				return nil
			}
			return headroompkg.StopBYOKGateway()
		},
		stopCopilotProxy: func() error {
			if isTest {
				return nil
			}
			return headroompkg.StopCopilotProxy()
		},
		stopGrokProxy: func() error {
			if isTest {
				return nil
			}
			return headroompkg.StopGrokOAuthProxy()
		},
		disableAutostart: func() error {
			if isTest {
				return nil
			}
			return headroompkg.DisableProxyAutostart()
		},
		removeAll: func(p string) error {
			if isTest {
				return nil
			}
			return os.RemoveAll(p)
		},
		removeFile: func(p string) error {
			if isTest {
				return nil
			}
			return os.Remove(p)
		},
		cleanUserPath: func(dir string) error {
			if isTest || !util.IsWin {
				return nil
			}
			return cleanWindowsUserPath(dir)
		},
		uninstallNpm: func(pkgs ...string) error {
			if isTest {
				return nil
			}
			npm := util.ResolveNpmBinary()
			if npm == "" {
				return nil
			}
			args := append([]string{"uninstall", "-g"}, pkgs...)
			res := util.Run(npm, args, util.RunOptions{Capture: true})
			if res.Code != 0 {
				return fmt.Errorf("npm uninstall: %s", res.Stderr)
			}
			return nil
		},
		uninstallRtk: func(p string) error {
			if isTest {
				return nil
			}
			if util.Exists(p) {
				_ = util.Run(p, []string{"init", "--uninstall"}, util.RunOptions{Capture: true})
				return os.Remove(p)
			}
			return nil
		},
		selfDelete: func(installDir string) error {
			if isTest {
				return nil
			}
			return selfDeleteDir(installDir)
		},
	}
}

// RunRemove completely unwires all agents, stops daemons, removes OS services,
// purges installed tool packages and data directories, and deletes tokless.
func RunRemove(opts InitOptions) int {
	return runRemoveWith(defaultRemoveOps(), opts)
}

func runRemoveWith(ops removeOps, opts InitOptions) int {
	util.L.Raw("")
	util.L.Raw("  " + util.C.Bold(util.C.Cyan("tokless")) + util.C.Gray("  remove (complete uninstaller)"))
	util.L.Raw("")

	if !opts.Yes && !opts.DryRun && util.IsInteractive() {
		util.L.Raw("  " + util.C.Yellow(util.Sym.Warn) + " " + util.C.Bold("This will completely remove Tokless from your system:"))
		util.L.Raw("    " + util.C.Gray("•") + " Unwire all 17 agent configurations (active & legacy)")
		util.L.Raw("    " + util.C.Gray("•") + " Remove MCP configs, hooks, Cursor rules, instructions, and plugins")
		util.L.Raw("    " + util.C.Gray("•") + " Stop background proxies (Headroom 8787, BYOK 18787, Copilot, Grok)")
		util.L.Raw("    " + util.C.Gray("•") + " Remove autostart scheduled tasks / services")
		util.L.Raw("    " + util.C.Gray("•") + " Uninstall tool packages (RTK, npm: codegraph, context-mode, projectmem)")
		util.L.Raw("    " + util.C.Gray("•") + " Delete Tokless data, cache, and state directories")
		util.L.Raw("    " + util.C.Gray("•") + " Remove Tokless binaries and installation directory")
		util.L.Raw("")
		if !util.Confirm("Are you sure you want to completely remove Tokless?", false) {
			util.L.Raw("  " + util.C.Gray("Aborted."))
			util.L.Raw("")
			return 0
		}
		util.L.Raw("")
	}

	if opts.DryRun {
		util.L.Raw("  " + util.C.Cyan(util.Sym.Info) + " " + util.C.Bold("[dry-run] Tokless complete removal plan:"))
		util.L.Raw("    " + util.C.Gray("1.") + " Stop local proxies (8787, 18787, Copilot, Grok) and remove autostart")
		util.L.Raw("    " + util.C.Gray("2.") + " Clean legacy artifacts (projectmem, caveman, ponytail, principles)")
		util.L.Raw("    " + util.C.Gray("3.") + " Unwire all active tools (rtk, codegraph, context-mode, headroom) from all agents")
		util.L.Raw("    " + util.C.Gray("4.") + " Strip Tokless instructions from all AGENTS.md / CLAUDE.md files")
		util.L.Raw("    " + util.C.Gray("5.") + " Remove Cursor rules (.cursor/rules/*.mdc) and OpenCode/Pi extensions")
		util.L.Raw("    " + util.C.Gray("6.") + " Uninstall RTK binary and npm globals (@colbymchenry/codegraph, context-mode, projectmem)")
		util.L.Raw("    " + util.C.Gray("7.") + " Delete data directory (" + util.ToklessDataDir() + ")")
		if rec, exact := util.InstallInfo(); exact && rec.Path != "" {
			util.L.Raw("    " + util.C.Gray("8.") + " Delete installation directory (" + filepath.Dir(rec.Path) + ")")
		} else if util.IsWin && os.Getenv("LOCALAPPDATA") != "" {
			util.L.Raw("    " + util.C.Gray("8.") + " Delete installation directory (" + filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "tokless") + ")")
		}
		util.L.Raw("")
		return 0
	}

	bar := util.NewSectionProgress("Removing Tokless")
	bar.Start(6)

	var failures []string

	// Phase 1: Stop proxies and disable autostart
	bar.Begin("Stopping background proxies & autostart")
	if err := ops.stopProxy(); err != nil {
		failures = append(failures, "proxy: "+err.Error())
	}
	if err := ops.stopBYOK(); err != nil {
		failures = append(failures, "byok: "+err.Error())
	}
	if err := ops.stopCopilotProxy(); err != nil {
		failures = append(failures, "copilot-proxy: "+err.Error())
	}
	if err := ops.stopGrokProxy(); err != nil {
		failures = append(failures, "grok-proxy: "+err.Error())
	}
	if err := ops.disableAutostart(); err != nil {
		failures = append(failures, "autostart: "+err.Error())
	}
	bar.Complete("")

	// Phase 2: Clean legacy artifacts (projectmem, caveman, ponytail, principles)
	bar.Begin("Cleaning legacy artifacts")
	_, legacyErrs := CleanLegacyArtifacts(false)
	for _, e := range legacyErrs {
		failures = append(failures, "legacy: "+e.Error())
	}
	bar.Complete("")

	// Phase 3: Unwire all agents (active tools + hooks + instructions + cursor rules)
	bar.Begin("Unwiring agent configurations")
	allTools := core.ListTools()
	for _, agent := range core.ListAgents() {
		for _, tool := range allTools {
			if unwire, ok := tool.UnwireFor[agent.ID]; ok {
				_, _ = unwire(core.RunOpts{Agent: agent.ID})
			}
		}
	}
	// Strip all instructions from all agents
	for _, agent := range core.ListAgents() {
		toolsPkg.RemoveOwner(agent.ID, "codegraph")
		toolsPkg.RemoveOwner(agent.ID, "context-mode")
		p := toolsPkg.InstructionPath(agent.ID)
		if p != "" && util.Exists(p) {
			raw, ok := util.ReadFileSafe(p)
			if ok {
				cleaned := toolsPkg.StripLegacyExported(raw)
				if isOnlyToklessHeaderContent(cleaned) {
					_ = os.Remove(p)
				}
			}
		}
	}
	CleanAllLegacyInstructions()
	// Clean Cursor rules and hooks
	CleanWorkspaceCursorRules()
	_ = agents.RemoveCursorProjectRulesHook()
	_ = agents.RemoveCursorCodegraphIndexHook()
	_ = agents.RemoveCursorRtkHook()
	// Clean OpenCode plugins
	agents.CleanOpenCodeLegacy()
	// Clean Pi packages and skills
	_ = agents.PiRemoveMcpAdapter()
	agents.CleanPiLegacy()
	// Deep sweep: purge any remaining Tokless MCP and hook configs
	deepSweepAgentConfigs()
	bar.Complete("")

	// Phase 4: Purge tool binaries & npm packages
	bar.Begin("Uninstalling tool binaries & packages")
	if p := util.ResolveRtkBin(); p != "" && util.Exists(p) {
		if err := ops.uninstallRtk(p); err != nil {
			failures = append(failures, "rtk: "+err.Error())
		}
	}
	if err := ops.uninstallNpm("@colbymchenry/codegraph", "context-mode", "projectmem"); err != nil {
		failures = append(failures, "npm: "+err.Error())
	}
	bar.Complete("")

	// Phase 5: Purge Tokless data, cache, and state
	bar.Begin("Purging Tokless data & cache directories")
	_ = ops.removeAll(util.HeadroomPathsResolved().Root)
	_ = ops.removeAll(util.ToklessDataDir())
	_ = ops.removeFile(util.InstallMarkerPath())
	_ = ops.removeFile(util.SelectionPath())
	bar.Complete("")

	// Phase 6: Clean Windows User PATH & remove binary installation directory
	bar.Begin("Removing binaries & installation directory")
	installDir := resolveToklessInstallDir()
	if installDir != "" {
		if util.IsWin {
			_ = ops.cleanUserPath(installDir)
		}
		if err := ops.selfDelete(installDir); err != nil {
			failures = append(failures, "self-delete: "+err.Error())
		}
	}
	bar.Complete("")
	bar.Done("")

	util.L.Raw("")
	if len(failures) > 0 && !opts.Force {
		util.L.Raw("  " + util.C.Yellow(util.Sym.Warn) + " " + util.C.Bold("Removal completed with notices:"))
		for _, f := range failures {
			util.L.Raw("    " + util.C.Gray("• ") + f)
		}
		util.L.Raw("")
		util.L.Raw("  " + util.C.Green(util.Sym.Check) + " " + util.C.Bold("Tokless wiring, daemons, and data have been removed."))
		util.L.Raw("")
		return 0
	}

	util.L.Raw("  " + util.C.Green(util.Sym.Check) + " " + util.C.Bold("Tokless has been completely removed."))
	util.L.Raw("")
	return 0
}

// resolveToklessInstallDir locates the directory where tokless is installed.
func resolveToklessInstallDir() string {
	if util.IsRunningInTest() {
		return ""
	}
	if rec, exact := util.InstallInfo(); exact && rec.Path != "" {
		return filepath.Dir(rec.Path)
	}
	if util.IsWin {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			d := filepath.Join(local, "Programs", "tokless")
			if util.Exists(d) {
				return d
			}
		}
	} else {
		d := filepath.Join(util.Home(), ".local", "bin")
		if util.Exists(filepath.Join(d, "tokless")) {
			return d
		}
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
			exe = resolved
		}
		if !util.IsGoTestExecutable(exe) {
			d := filepath.Dir(exe)
			if filepath.Base(d) == "tokless" {
				return d
			}
		}
	}
	return ""
}

// selfDeleteDir removes the installation directory or schedules self-deletion on Windows.
func selfDeleteDir(installDir string) error {
	if util.IsRunningInTest() || installDir == "" || !util.Exists(installDir) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return os.RemoveAll(installDir)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	installDirClean := filepath.Clean(installDir)
	exeDirClean := filepath.Clean(filepath.Dir(exe))

	// If we are not running from within installDir, remove directly.
	if !strings.EqualFold(exeDirClean, installDirClean) &&
		!strings.HasPrefix(strings.ToLower(exe), strings.ToLower(installDirClean)+string(os.PathSeparator)) {
		return os.RemoveAll(installDir)
	}

	// On Unix, an unlinked executable continues running until exit.
	if !util.IsWin {
		_ = os.Remove(exe)
		_ = os.Remove(filepath.Join(installDir, "toklessw"))
		return nil
	}

	// On Windows, remove all other files in installDir first.
	entries, _ := os.ReadDir(installDir)
	for _, e := range entries {
		p := filepath.Join(installDir, e.Name())
		if strings.EqualFold(p, exe) {
			continue
		}
		_ = os.RemoveAll(p)
	}

	// Spawn detached cmd to wait ~1.5s for process exit then remove directory.
	cmdStr := fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & rmdir /s /q "%s"`, installDir)
	bgCmd := exec.Command("cmd.exe", "/C", cmdStr)
	util.PrepareSysProcAttr(bgCmd, true)
	return bgCmd.Start()
}

// cleanWindowsUserPath removes the given directory from HKCU\Environment\Path.
func cleanWindowsUserPath(dir string) error {
	if util.IsRunningInTest() || !util.IsWin || dir == "" {
		return nil
	}
	dir = strings.TrimRight(dir, `\/`)
	psCmd := fmt.Sprintf(`
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
if ($key) {
    $p = $key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    if ($p) {
        $parts = $p -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ne '%s' }
        $key.SetValue('Path', ($parts -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)
    }
    $key.Close()
}
`, strings.ReplaceAll(dir, `'`, `''`))

	res := util.Run("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", psCmd}, util.RunOptions{Capture: true})
	if res.Code != 0 {
		return fmt.Errorf("clean path: %s", res.Stderr)
	}
	return nil
}

// deepSweepAgentConfigs checks all agent configuration files and prunes any remaining
// entries referencing "tokless" or "run-mcp" or Tokless-managed tools.
func deepSweepAgentConfigs() {
	cleanJsonMcp := func(filePath string, childKey string) {
		if raw, ok := util.ReadFileSafe(filePath); ok {
			if cfg := util.TryParseJsonc(raw); cfg != nil {
				changed := false
				targetMap := cfg
				if childKey != "" {
					if strings.Contains(childKey, ".") {
						parts := strings.Split(childKey, ".")
						curr := cfg
						for _, p := range parts {
							if next, ok := curr.Get(p); ok {
								if om, ok := next.(*util.OrderedMap); ok && om != nil {
									curr = om
								} else {
									return
								}
							} else {
								return
							}
						}
						targetMap = curr
					} else {
						if cm, ok := cfg.Get(childKey); ok {
							if om, ok := cm.(*util.OrderedMap); ok && om != nil {
								targetMap = om
							} else {
								return
							}
						} else {
							return
						}
					}
				}
				for _, k := range targetMap.Keys() {
					v, _ := targetMap.Get(k)
					if k == "codegraph" || k == "context-mode" || k == "context_mode" || k == "projectmem" || k == "caveman" || isToklessManagedEntry(v) {
						targetMap.Delete(k)
						changed = true
					}
				}
				if changed {
					_ = util.WriteFile(filePath, util.StringifyJSON(cfg))
				}
			}
		}
	}

	// 1. Claude
	cleanJsonMcp(util.ClaudeCodePaths().GlobalJSON, "mcpServers")

	// 2. Codex
	codexFile := util.CodexPathsResolved().Config
	if raw, ok := util.ReadFileSafe(codexFile); ok {
		next := raw
		for _, tool := range []string{"codegraph", "context-mode", "context_mode", "projectmem", "headroom"} {
			next = util.RemoveBlock(next, "mcp_servers."+tool)
		}
		if next != raw {
			_ = util.WriteFile(codexFile, next)
		}
	}
	codexHooks := agents.CodexRtkHooksFile()
	if raw, ok := util.ReadFileSafe(codexHooks); ok {
		if cfg := util.TryParseJsonc(raw); cfg != nil {
			changed := false
			if hooks, ok := cfg.Get("hooks"); ok {
				if hm, ok := hooks.(*util.OrderedMap); ok && hm != nil {
					for _, event := range hm.Keys() {
						v, _ := hm.Get(event)
						arr, ok := v.([]any)
						if !ok {
							continue
						}
						var kept []any
						for _, g := range arr {
							if isToklessManagedEntry(g) {
								changed = true
								continue
							}
							if gm, ok := g.(*util.OrderedMap); ok && gm != nil {
								if hooksVal, ok := gm.Get("hooks"); ok {
									if hooksArr, ok := hooksVal.([]any); ok {
										var keptSub []any
										for _, sub := range hooksArr {
											if isToklessManagedEntry(sub) {
												changed = true
												continue
											}
											keptSub = append(keptSub, sub)
										}
										if len(keptSub) != len(hooksArr) {
											gm.Set("hooks", keptSub)
											changed = true
										}
										if len(keptSub) == 0 {
											changed = true
											continue
										}
									}
								}
							}
							kept = append(kept, g)
						}
						if len(kept) != len(arr) {
							hm.Set(event, kept)
							changed = true
						}
					}
				}
			}
			if changed {
				_ = util.WriteFile(codexHooks, util.StringifyJSON(cfg))
			}
		}
	}

	// 3. Antigravity / Gemini
	cleanJsonMcp(util.AntigravityPathsResolved().McpConfigCLI, "mcpServers")
	cleanJsonMcp(util.AntigravityPathsResolved().McpConfig, "mcpServers")
	cleanJsonMcp(filepath.Join(util.Home(), ".gemini", "antigravity-cli", "mcp_config.json"), "mcpServers")

	// 4. GitHub Copilot
	cleanJsonMcp(util.CopilotPathsResolved().McpConfig, "mcpServers")

	// 5. OpenCode & OpenCode2
	cleanJsonMcp(util.OpenCodePathsResolved().Config, "mcp")
	cleanJsonMcp(util.OpenCodePathsV2().Config, "mcp.servers")

	// 6. Droid, Pi, OMP, Cline, Kimi, Kilo, Amp, Zcode
	cleanJsonMcp(filepath.Join(util.Home(), ".factory", "mcp.json"), "mcpServers")
	cleanJsonMcp(filepath.Join(agents.PiAgentDirResolved(), "mcp.json"), "mcpServers")
	cleanJsonMcp(filepath.Join(agents.OmpAgentDirResolved(), "mcp.json"), "mcpServers")
	cleanJsonMcp(util.ClinePathsResolved().McpConfig, "mcpServers")
	cleanJsonMcp(util.KimiPathsResolved().McpConfig, "mcpServers")
	cleanJsonMcp(util.KiloPathsResolved().Config, "mcp")
	cleanJsonMcp(util.AmpPathsResolved().Config, "amp.mcpServers")
	cleanJsonMcp(util.ZcodePathsResolved().McpConfig, "mcp.servers")
}

func isOnlyToklessHeaderContent(s string) bool {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || t == "# Agent Instructions" || t == "# Agent Operating System" || t == "## Index" || t == "## Index →" || t == "Apply on every coding task:" || t == "**Tools (call):**" || t == "**Skills (apply):**" {
			continue
		}
		return false
	}
	return true
}

func isToklessManagedEntry(v any) bool {
	if util.IsToklessOrToolMcpEntry("codegraph", v) || util.IsToklessOrToolMcpEntry("context-mode", v) || util.IsToklessOrToolMcpEntry("projectmem", v) || util.IsToklessOrToolMcpEntry("headroom", v) {
		return true
	}
	if m, ok := v.(*util.OrderedMap); ok && m != nil {
		if cmd, ok := m.Get("command"); ok {
			if s, ok := cmd.(string); ok && (strings.Contains(strings.ToLower(s), "tokless") || strings.Contains(strings.ToLower(s), "codegraph") || strings.Contains(strings.ToLower(s), "context-mode")) {
				return true
			}
		}
		if args, ok := m.Get("args"); ok {
			if arr, ok := args.([]any); ok {
				for _, a := range arr {
					if s, ok := a.(string); ok && (s == "run-mcp" || strings.Contains(s, "tokless") || strings.Contains(s, "codegraph") || strings.Contains(s, "context-mode")) {
						return true
					}
				}
			}
		}
	}
	return false
}
