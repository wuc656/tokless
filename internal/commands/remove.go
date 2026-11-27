package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/core"
	headroompkg "github.com/HoangP8/tokless/internal/headroom"
	toolsPkg "github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

type removeOps struct {
	stopProxy         func() error
	stopBYOK          func() error
	stopCopilotProxy  func() error
	stopGrokProxy     func() error
	disableAutostart  func() error
	removeAll         func(string) error
	removeFile        func(string) error
	resolveInstallDir func() string
	cleanUserPath     func(string) error
	uninstallNpm      func(pkgs ...string) error
	uninstallRtk      func(string) error
	selfDelete        func(string) error
}

func defaultRemoveOps() removeOps {
	isTest := util.IsRunningInTest()
	return removeOps{
		stopProxy: func() error {
			if isTest {
				return nil
			}
			return headroompkg.PurgeStopProxy()
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
		resolveInstallDir: resolveToklessInstallDir,
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

// CleanResetPurge cleans all background proxies, legacy artifacts, agent wiring,
// installed tool packages, and data directories while preserving tokless itself.
func CleanResetPurge(opts InitOptions) []string {
	return cleanResetPurgeWith(defaultRemoveOps(), opts)
}

func cleanResetPurgeWith(ops removeOps, opts InitOptions) []string {
	bar := util.NewSectionProgress("Fresh install reset")
	bar.Start(5)

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
				if _, err := unwire(core.RunOpts{Agent: agent.ID}); err != nil && !os.IsNotExist(err) {
					failures = append(failures, "unwire "+agent.ID+"/"+tool.ID+": "+err.Error())
				}
			}
		}
	}
	// Strip all instructions from all agents
	for _, agent := range core.ListAgents() {
		for _, owner := range util.ToklessOwners {
			toolsPkg.RemoveOwner(agent.ID, owner)
		}
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
	if p := util.ResolveExistingRtkBin(); p != "" && util.Exists(p) {
		if err := ops.uninstallRtk(p); err != nil {
			failures = append(failures, "rtk: "+err.Error())
		}
	}
	if err := ops.uninstallNpm("@colbymchenry/codegraph", "context-mode", "projectmem"); err != nil {
		failures = append(failures, "npm: "+err.Error())
	}
	bar.Complete("")

	// Phase 5: Purge Tokless data, cache, and state (KEEP tokless binary!)
	bar.Begin("Purging Tokless data & cache directories")
	_ = ops.removeAll(util.HeadroomPathsResolved().Root)
	_ = ops.removeAll(util.ToklessDataDir())
	_ = ops.removeFile(util.InstallMarkerPath())
	_ = ops.removeFile(util.SelectionPath())
	bar.Complete("")
	bar.Done("")

	return failures
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
				if _, err := unwire(core.RunOpts{Agent: agent.ID}); err != nil && !os.IsNotExist(err) {
					failures = append(failures, "unwire "+agent.ID+"/"+tool.ID+": "+err.Error())
				}
			}
		}
	}
	// Strip all instructions from all agents
	for _, agent := range core.ListAgents() {
		for _, owner := range util.ToklessOwners {
			toolsPkg.RemoveOwner(agent.ID, owner)
		}
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
	if p := util.ResolveExistingRtkBin(); p != "" && util.Exists(p) {
		if err := ops.uninstallRtk(p); err != nil {
			failures = append(failures, "rtk: "+err.Error())
		}
	}
	if err := ops.uninstallNpm("@colbymchenry/codegraph", "context-mode", "projectmem"); err != nil {
		failures = append(failures, "npm: "+err.Error())
	}
	bar.Complete("")

	// Resolve installation dir before Phase 5 deletes InstallMarkerPath.
	resolveInstallDir := ops.resolveInstallDir
	if resolveInstallDir == nil {
		resolveInstallDir = resolveToklessInstallDir
	}
	installDir := resolveInstallDir()

	// Phase 5: Purge Tokless data, cache, and state
	bar.Begin("Purging Tokless data & cache directories")
	for _, path := range []string{util.HeadroomPathsResolved().Root, util.ToklessDataDir()} {
		if path != "" && util.Exists(path) {
			if err := ops.removeAll(path); err != nil && !os.IsNotExist(err) {
				failures = append(failures, "remove "+filepath.Base(path)+": "+err.Error())
			}
		}
	}
	for _, path := range []string{util.InstallMarkerPath(), util.SelectionPath()} {
		if path != "" && util.Exists(path) {
			if err := ops.removeFile(path); err != nil && !os.IsNotExist(err) {
				failures = append(failures, "remove "+filepath.Base(path)+": "+err.Error())
			}
		}
	}
	bar.Complete("")

	// Phase 6: Clean Windows User PATH & remove binary installation directory
	bar.Begin("Removing binaries & installation directory")
	if installDir != "" {
		if util.IsWin && isToklessInstallationDir(installDir) {
			if err := ops.cleanUserPath(installDir); err != nil {
				failures = append(failures, "clean-path: "+err.Error())
			}
		}
		if err := ops.selfDelete(installDir); err != nil {
			failures = append(failures, "self-delete: "+err.Error())
		}
	}
	bar.Complete("")
	bar.Done("")

	util.L.Raw("")
	if len(failures) > 0 && !opts.Force {
		util.L.Err("Removal completed with failures:")
		for _, f := range failures {
			util.L.Raw("    " + util.C.Gray("• ") + f)
		}
		util.L.Raw("")
		return 1
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

var toklessArtifactRe = regexp.MustCompile(`^(?:toklessw?(?:\.exe)?)\.(?:old(?:\.\d+)?|\d+\.bak|download(?:-\d+)?)$`)

func isKnownToklessInstallArtifact(name string, isDir bool) bool {
	if isDir {
		return false
	}
	lower := strings.ToLower(name)
	switch lower {
	case "tokless", "tokless.exe", "toklessw", "toklessw.exe", "install.json", "versions.json", ".tokless-selfupdate.lock":
		return true
	}
	return toklessArtifactRe.MatchString(lower)
}

// isToklessInstallationDir checks whether dir contains only known tokless artifacts
// and has a "tokless" directory name so recursive removal never touches user dirs.
func isToklessInstallationDir(dir string) bool {
	if dir == "" {
		return false
	}
	if strings.ToLower(filepath.Base(dir)) != "tokless" {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !isKnownToklessInstallArtifact(e.Name(), e.IsDir()) {
			return false
		}
	}
	return true
}

// removeToklessBinariesInDir deletes only verified Tokless executables inside a
// shared bin directory or an install directory that also holds user subdirectories/files.
func removeToklessBinariesInDir(installDir string) error {
	exe, _ := os.Executable()
	if exe != "" {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
			exe = resolved
		}
		exe = filepath.Clean(exe)
	}
	for _, name := range []string{"tokless", "toklessw", "tokless.exe", "toklessw.exe"} {
		p := filepath.Join(installDir, name)
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		if util.IsWin && exe != "" && strings.EqualFold(filepath.Clean(p), exe) {
			cmdStr := fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & del /f /q "%s"`, p)
			bgCmd := exec.Command("cmd.exe", "/C", cmdStr)
			util.PrepareSysProcAttr(bgCmd, true)
			if err := bgCmd.Start(); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// selfDeleteDir removes a dedicated installation directory or only the Tokless binaries in a shared directory.
func selfDeleteDir(installDir string) error {
	if util.IsRunningInTest() || installDir == "" || !util.Exists(installDir) {
		return nil
	}
	if !isToklessInstallationDir(installDir) {
		return removeToklessBinariesInDir(installDir)
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
	cleanJsonMcp := func(filePath string, path ...string) {
		raw, ok := util.ReadFileSafe(filePath)
		if !ok {
			return
		}
		cfg := util.TryParseJsonc(raw)
		if cfg == nil {
			return
		}
		targetMap := cfg
		for _, p := range path {
			nextVal, has := targetMap.Get(p)
			if !has {
				return
			}
			om, isMap := nextVal.(*util.OrderedMap)
			if !isMap || om == nil {
				return
			}
			targetMap = om
		}
		next := raw
		changed := false
		for _, k := range targetMap.Keys() {
			v, _ := targetMap.Get(k)
			if !util.IsToklessManagedMcpEntry(v) {
				continue
			}
			keyPath := append(append([]string(nil), path...), k)
			edited, deleted, err := util.DeleteJSONCNestedKey(next, keyPath)
			if err != nil {
				return
			}
			if deleted {
				next = edited
				changed = true
			}
		}
		if changed && next != raw {
			_ = util.WriteFile(filePath, next)
		}
	}

	// 1. Claude
	cleanJsonMcp(util.ClaudeCodePaths().GlobalJSON, "mcpServers")

	// 2. Codex
	codexFile := util.CodexPathsResolved().Config
	if raw, ok := util.ReadFileSafe(codexFile); ok {
		next := raw
		for _, tool := range []string{"codegraph", "context-mode", "context_mode", "projectmem", "headroom"} {
			header := "mcp_servers." + tool
			if util.IsToklessManagedTomlMcpBlock(next, header, "") {
				next = util.RemoveBlock(next, header)
			}
		}
		if next != raw {
			_ = util.WriteFile(codexFile, next)
		}
	}
	codexHooks := agents.CodexRtkHooksFile()
	if raw, ok := util.ReadFileSafe(codexHooks); ok {
		if cfg := util.TryParseJsonc(raw); cfg != nil {
			next := raw
			changed := false
			if hooks, ok := cfg.Get("hooks"); ok {
				if hm, ok := hooks.(*util.OrderedMap); ok && hm != nil {
					for _, event := range hm.Keys() {
						v, _ := hm.Get(event)
						arr, ok := v.([]any)
						if !ok {
							continue
						}
						removeGroups := map[int]bool{}
						for gi := len(arr) - 1; gi >= 0; gi-- {
							g := arr[gi]
							if isToklessManagedHookEntry(g) {
								removeGroups[gi] = true
								continue
							}
							gm, ok := g.(*util.OrderedMap)
							if !ok || gm == nil {
								continue
							}
							hooksVal, ok := gm.Get("hooks")
							if !ok {
								continue
							}
							hooksArr, ok := hooksVal.([]any)
							if !ok || len(hooksArr) == 0 {
								continue
							}
							subRemove := map[int]bool{}
							for si, sub := range hooksArr {
								if isToklessManagedHookEntry(sub) {
									subRemove[si] = true
								}
							}
							if len(subRemove) == 0 {
								continue
							}
							if len(subRemove) >= len(hooksArr) {
								removeGroups[gi] = true
								continue
							}
							edited, deleted, err := util.DeleteJSONCSubArrayIndices(next, []string{"hooks", event}, gi, "hooks", subRemove)
							if err != nil {
								next = raw
								changed = false
								break
							}
							if deleted {
								next = edited
								changed = true
							}
						}
						if len(removeGroups) > 0 {
							edited, deleted, err := util.DeleteJSONCArrayIndices(next, []string{"hooks", event}, removeGroups)
							if err != nil {
								next = raw
								changed = false
								break
							}
							if deleted {
								next = edited
								changed = true
							}
						}
					}
				}
			}
			if changed && next != raw {
				_ = util.WriteFile(codexHooks, next)
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
	cleanJsonMcp(util.OpenCodePathsV2().Config, "mcp", "servers")

	// 6. Droid, Pi, OMP, Cline, Kimi, Kilo, Amp, Zcode
	cleanJsonMcp(filepath.Join(util.Home(), ".factory", "mcp.json"), "mcpServers")
	cleanJsonMcp(filepath.Join(agents.PiAgentDirResolved(), "mcp.json"), "mcpServers")
	cleanJsonMcp(filepath.Join(agents.OmpAgentDirResolved(), "mcp.json"), "mcpServers")
	cleanJsonMcp(util.ClinePathsResolved().McpConfig, "mcpServers")
	cleanJsonMcp(util.KimiPathsResolved().McpConfig, "mcpServers")
	cleanJsonMcp(util.KiloPathsResolved().Config, "mcp")
	cleanJsonMcp(util.AmpPathsResolved().Config, "amp.mcpServers")
	cleanJsonMcp(util.ZcodePathsResolved().McpConfig, "mcp", "servers")
}

func isOnlyToklessHeaderContent(s string) bool {
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || t == "# Agent Instructions" || t == "# Agent Operating System" || t == "## Index" || t == "## Index →" || t == "Apply on every coding task:" || t == "Token-saving defaults (preserve normal reasoning and behavior):" || t == "**Tools (call):**" || t == "**Skills (apply):**" || t == "**Discipline (apply):**" {
			continue
		}
		return false
	}
	return true
}

func isToklessManagedHookEntry(v any) bool {
	m, ok := v.(*util.OrderedMap)
	if !ok || m == nil {
		return false
	}
	if _, hasArgs := m.Get("args"); hasArgs {
		return false
	}
	cmd, ok := m.Get("command")
	if !ok {
		return false
	}
	cmdStr, ok := cmd.(string)
	if !ok {
		return false
	}
	fields := util.SplitCommand(strings.TrimSpace(cmdStr))
	if len(fields) < 2 {
		return false
	}
	exe := strings.Trim(fields[0], `"'`)
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(exe, "\\", "/")))
	if util.IsToklessCommand(exe) || strings.HasPrefix(base, "tokless-") {
		switch fields[1] {
		case "rtk-hook", "codex-perm", "run-mcp":
			return true
		}
	}
	baseNoExt := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".exe"), ".cmd"), ".bat")
	if baseNoExt == "context-mode" && fields[1] == "hook" {
		return true
	}
	return false
}
