package agents

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/util"
)

// CleanClaudeLegacy cleans legacy ProjectMem MCP and permissions from Claude configs.
func CleanClaudeLegacy() bool {
	p := util.ClaudeCodePaths()
	changed := false
	if raw, ok := util.ReadFileSafe(p.GlobalJSON); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
			if servers, ok := cfg.Get("mcpServers"); ok {
				if sm, ok := servers.(*util.OrderedMap); ok && sm != nil {
					if _, has := sm.Get("projectmem"); has {
						sm.Delete("projectmem")
						changed = true
					}
				}
			}
			if perms, ok := cfg.Get("permissions"); ok {
				if pm, ok := perms.(*util.OrderedMap); ok && pm != nil {
					if v, ok := pm.Get("allow"); ok {
						if arr, ok := v.([]any); ok {
							var kept []any
							for _, e := range arr {
								if s, ok := e.(string); ok && (strings.Contains(s, "projectmem") || strings.Contains(s, "caveman")) {
									changed = true
									continue
								}
								kept = append(kept, e)
							}
							if len(kept) != len(arr) {
								pm.Set("allow", kept)
								changed = true
							}
						}
					}
				}
			}
			if changed {
				_ = util.WriteFile(p.GlobalJSON, util.StringifyJSON(cfg))
			}
		}
	}
	if raw, ok := util.ReadFileSafe(p.Settings); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
			if perms, ok := cfg.Get("permissions"); ok {
				if pm, ok := perms.(*util.OrderedMap); ok && pm != nil {
					if v, ok := pm.Get("allow"); ok {
						if arr, ok := v.([]any); ok {
							var kept []any
							for _, e := range arr {
								if s, ok := e.(string); ok && (strings.Contains(s, "projectmem") || strings.Contains(s, "caveman")) {
									continue
								}
								kept = append(kept, e)
							}
							if len(kept) != len(arr) {
								pm.Set("allow", kept)
								_ = util.WriteFile(p.Settings, util.StringifyJSON(cfg))
								changed = true
							}
						}
					}
				}
			}
		}
	}
	return changed
}

// CleanCodexLegacy cleans legacy ProjectMem hooks and MCP blocks from Codex configs.
func CleanCodexLegacy() bool {
	changed := false
	hooksFile := codexHooksFile()
	if raw, ok := util.ReadFileSafe(hooksFile); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
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
							gm, ok := g.(*util.OrderedMap)
							if !ok {
								kept = append(kept, g)
								continue
							}
							if isLegacyCodexHookGroup(gm) {
								changed = true
								continue
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
				_ = util.WriteFile(hooksFile, util.StringifyJSON(cfg))
			}
		}
	}
	p := util.CodexPathsResolved()
	if raw, ok := util.ReadFileSafe(p.Config); ok {
		next := raw
		if strings.Contains(next, "mcp_servers.projectmem") {
			next = util.RemoveBlock(next, "mcp_servers.projectmem")
			changed = true
		}
		next = sweepStaleHookStateEntries(next)
		if next != raw {
			_ = util.WriteFile(p.Config, next)
			changed = true
		}
	}
	return changed
}

func isLegacyCodexHookGroup(gm *util.OrderedMap) bool {
	if cmd, ok := gm.Get("command"); ok {
		if s, ok := cmd.(string); ok && strings.Contains(s, "projectmem-hook") {
			return true
		}
	}
	if hooksVal, ok := gm.Get("hooks"); ok {
		if hooksArr, ok := hooksVal.([]any); ok {
			for _, h := range hooksArr {
				if hm, ok := h.(*util.OrderedMap); ok {
					if cmd, ok := hm.Get("command"); ok {
						if s, ok := cmd.(string); ok && strings.Contains(s, "projectmem-hook") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// CleanAntigravityLegacy cleans legacy ProjectMem hooks and MCP configs from Antigravity.
func CleanAntigravityLegacy() bool {
	changed := false
	hooksFile := antigravityHooksFile()
	if raw, ok := util.ReadFileSafe(hooksFile); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
			if _, ok := cfg.Get("projectmem-context"); ok {
				cfg.Delete("projectmem-context")
				changed = true
			}
			if _, ok := cfg.Get("tokless-projectmem"); ok {
				cfg.Delete("tokless-projectmem")
				changed = true
			}
			if changed {
				_ = util.WriteFile(hooksFile, util.StringifyJSON(cfg))
			}
		}
	}
	files := append(antigravityMcpConfigFiles(), antigravityLegacyMcpFiles()...)
	for _, f := range files {
		if raw, ok := util.ReadFileSafe(f); ok {
			cfg := util.TryParseJsonc(raw)
			if cfg == nil {
				continue
			}
			fileChanged := false
			if s, ok := cfg.Get("mcpServers"); ok {
				if sm, ok := s.(*util.OrderedMap); ok && sm != nil {
					if _, has := sm.Get("projectmem"); has {
						sm.Delete("projectmem")
						fileChanged = true
					}
				}
			}
			if p, ok := cfg.Get("permissions"); ok {
				if pm, ok := p.(*util.OrderedMap); ok && pm != nil {
					if v, ok := pm.Get("allow"); ok {
						if arr, ok := v.([]any); ok {
							var kept []any
							for _, e := range arr {
								if s, ok := e.(string); ok && (strings.Contains(s, "projectmem") || strings.Contains(s, "caveman")) {
									fileChanged = true
									continue
								}
								kept = append(kept, e)
							}
							if len(kept) != len(arr) {
								pm.Set("allow", kept)
								fileChanged = true
							}
						}
					}
				}
			}
			if fileChanged {
				_ = util.WriteFile(f, util.StringifyJSON(cfg))
				changed = true
			}
		}
	}
	return changed
}

// CleanCopilotLegacy cleans legacy ProjectMem hooks and MCP entries from Copilot.
func CleanCopilotLegacy() bool {
	changed := false
	hookFiles := []string{
		copilotHooksFile("projectmem-context.json"),
		copilotHooksFile("tokless-projectmem.json"),
		copilotIdeHooksFile("projectmem-context.json"),
		copilotIdeHooksFile("tokless-projectmem.json"),
	}
	for _, f := range hookFiles {
		if util.Exists(f) {
			_ = os.Remove(f)
			changed = true
		}
	}
	mcpFiles := []string{
		util.CopilotPathsResolved().McpConfig,
		copilotIdeMcpFile(),
	}
	for _, f := range mcpFiles {
		if raw, ok := util.ReadFileSafe(f); ok {
			cfg := util.TryParseJsonc(raw)
			if cfg != nil {
				if servers, ok := cfg.Get("mcpServers"); ok {
					if sm, ok := servers.(*util.OrderedMap); ok && sm != nil {
						if _, has := sm.Get("projectmem"); has {
							sm.Delete("projectmem")
							_ = util.WriteFile(f, util.StringifyJSON(cfg))
							changed = true
						}
					}
				}
			}
		}
	}
	return changed
}

// CleanDroidLegacy cleans legacy ProjectMem hooks and MCP from Droid.
func CleanDroidLegacy() bool {
	changed := false
	raw := droidRawHooks()
	cfg := droidHooksLoad()
	if cfg != nil {
		if hm, ok := cfg.Get("hooks"); ok {
			if hom, ok := hm.(*util.OrderedMap); ok && hom != nil {
				for _, event := range hom.Keys() {
					v, _ := hom.Get(event)
					arr, ok := v.([]any)
					if !ok {
						continue
					}
					var kept []any
					for _, g := range arr {
						gm, ok := g.(*util.OrderedMap)
						if !ok {
							kept = append(kept, g)
							continue
						}
						if cmdVal, ok := gm.Get("command"); ok {
							if s, ok := cmdVal.(string); ok && strings.Contains(s, "projectmem-hook") {
								changed = true
								continue
							}
						}
						kept = append(kept, g)
					}
					if len(kept) != len(arr) {
						hom.Set(event, kept)
						changed = true
					}
				}
			}
		}
		if changed {
			droidHooksSave(cfg, raw)
		}
	}
	mcpFile := filepath.Join(util.Home(), ".factory", "mcp.json")
	if raw, ok := util.ReadFileSafe(mcpFile); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
			if servers, ok := cfg.Get("mcpServers"); ok {
				if sm, ok := servers.(*util.OrderedMap); ok && sm != nil {
					if _, has := sm.Get("projectmem"); has {
						sm.Delete("projectmem")
						_ = util.WriteFile(mcpFile, util.StringifyJSON(cfg))
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// CleanOpenCodeLegacy cleans legacy caveman plugins, cavecrew agents, and projectmem MCP from OpenCode.
func CleanOpenCodeLegacy() bool {
	changed := false
	configs := []string{
		util.OpenCodePathsResolved().Config,
		util.OpenCodePathsV2().Config,
	}
	for _, f := range configs {
		if raw, ok := util.ReadFileSafe(f); ok {
			cfg := util.TryParseJsonc(raw)
			if cfg == nil {
				continue
			}
			fileChanged := false
			if mcp, ok := cfg.Get("mcp"); ok {
				if mm, ok := mcp.(*util.OrderedMap); ok && mm != nil {
					if _, has := mm.Get("projectmem"); has {
						mm.Delete("projectmem")
						fileChanged = true
					}
					if srv, ok := mm.Get("servers"); ok {
						if sm, ok := srv.(*util.OrderedMap); ok && sm != nil {
							if _, has := sm.Get("projectmem"); has {
								sm.Delete("projectmem")
								fileChanged = true
							}
						}
					}
				}
			}
			if plugin, ok := cfg.Get("plugin"); ok {
				if arr, ok := plugin.([]any); ok {
					var kept []any
					for _, p := range arr {
						if s, ok := p.(string); ok && (strings.Contains(s, "caveman") || strings.Contains(s, "cavecrew")) {
							fileChanged = true
							continue
						}
						kept = append(kept, p)
					}
					if len(kept) != len(arr) {
						cfg.Set("plugin", kept)
						fileChanged = true
					}
				}
			}
			if fileChanged {
				_ = util.WriteFile(f, util.StringifyJSON(cfg))
				changed = true
			}
		}
	}
	for _, baseDir := range []string{
		filepath.Dir(util.OpenCodePathsResolved().Config),
		filepath.Dir(util.OpenCodePathsV2().Config),
	} {
		cavemanPluginDir := filepath.Join(baseDir, "plugins", "caveman")
		if util.Exists(cavemanPluginDir) {
			_ = os.RemoveAll(cavemanPluginDir)
			changed = true
		}
		agentsDir := filepath.Join(baseDir, "agents")
		if entries, err := os.ReadDir(agentsDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasPrefix(e.Name(), "cavecrew-") && strings.HasSuffix(e.Name(), ".md") {
					_ = os.Remove(filepath.Join(agentsDir, e.Name()))
					changed = true
				}
			}
		}
	}
	return changed
}

// CleanGrokLegacy cleans legacy ProjectMem MCP block from Grok TOML config.
func CleanGrokLegacy() bool {
	changed := false
	configFile := filepath.Join(util.Home(), ".grok", "config.toml")
	if raw, ok := util.ReadFileSafe(configFile); ok {
		if strings.Contains(raw, "mcp_servers.projectmem") {
			next := util.RemoveBlock(raw, "mcp_servers.projectmem")
			if next != raw {
				_ = util.WriteFile(configFile, next)
				changed = true
			}
		}
	}
	return changed
}

// CleanPiLegacy cleans legacy Pi packages and skills for caveman, ponytail, and projectmem.
func CleanPiLegacy() bool {
	changed := false
	for _, src := range []string{"npm:pi-caveman", "npm:pi-ponytail"} {
		if PiSourceHas(src) {
			if PiRemoveSource(src) {
				changed = true
			}
		}
	}
	mcpFile := filepath.Join(PiAgentDirResolved(), "mcp.json")
	if raw, ok := util.ReadFileSafe(mcpFile); ok {
		cfg := util.TryParseJsonc(raw)
		if cfg != nil {
			if servers, ok := cfg.Get("mcpServers"); ok {
				if sm, ok := servers.(*util.OrderedMap); ok && sm != nil {
					if _, has := sm.Get("projectmem"); has {
						sm.Delete("projectmem")
						_ = util.WriteFile(mcpFile, util.StringifyJSON(cfg))
						changed = true
					}
				}
			}
		}
	}
	skillsDir := PiSkillsDir()
	for _, name := range []string{"caveman", "ponytail", "ponytail-audit", "ponytail-debt", "ponytail-gain", "ponytail-help", "ponytail-review"} {
		dir := filepath.Join(skillsDir, name)
		if util.Exists(dir) {
			_ = os.RemoveAll(dir)
			changed = true
		}
	}
	return changed
}

// CleanCursorLegacy cleans legacy ProjectMem MCP and legacy Cursor rules.
func CleanCursorLegacy() bool {
	changed := false
	for _, target := range cursorMcpTargets() {
		if raw, ok := util.ReadFileSafe(target.path); ok {
			cfg := util.TryParseJsonc(raw)
			if cfg != nil {
				if servers, ok := cursorMcpMapRead(cfg); ok && servers != nil {
					if _, has := servers.Get("projectmem"); has {
						servers.Delete("projectmem")
						_ = util.WriteFile(target.path, util.StringifyJSON(cfg))
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// CleanGenericMcp removes projectmem from any agent MCP config.
func CleanGenericMcp(mcpPath, jsonKey string) bool {
	if mcpPath == "" || !util.Exists(mcpPath) {
		return false
	}
	raw, ok := util.ReadFileSafe(mcpPath)
	if !ok {
		return false
	}
	if strings.HasSuffix(mcpPath, ".toml") || strings.HasSuffix(mcpPath, ".yml") || strings.HasSuffix(mcpPath, ".yaml") {
		if strings.Contains(raw, "projectmem") {
			next := util.RemoveBlock(raw, "mcp_servers.projectmem")
			if next != raw {
				_ = util.WriteFile(mcpPath, next)
				return true
			}
		}
		return false
	}
	cfg := util.TryParseJsonc(raw)
	if cfg == nil {
		return false
	}
	parts := strings.Split(jsonKey, ".")
	cur := cfg
	for i := 0; i < len(parts)-1; i++ {
		v, ok := cur.Get(parts[i])
		if !ok {
			return false
		}
		next, ok := v.(*util.OrderedMap)
		if !ok || next == nil {
			return false
		}
		cur = next
	}
	targetKey := parts[len(parts)-1]
	if _, has := cur.Get(targetKey); has {
		cur.Delete(targetKey)
		_ = util.WriteFile(mcpPath, util.StringifyJSON(cfg))
		return true
	}
	return false
}

// CleanAllAgentsLegacy cleans legacy artifacts across all 17 agents.
func CleanAllAgentsLegacy() []string {
	var cleaned []string
	if CleanClaudeLegacy() {
		cleaned = append(cleaned, "claude")
	}
	if CleanCodexLegacy() {
		cleaned = append(cleaned, "codex")
	}
	if CleanAntigravityLegacy() {
		cleaned = append(cleaned, "antigravity")
	}
	if CleanCopilotLegacy() {
		cleaned = append(cleaned, "copilot")
	}
	if CleanDroidLegacy() {
		cleaned = append(cleaned, "droid")
	}
	if CleanOpenCodeLegacy() {
		cleaned = append(cleaned, "opencode")
	}
	if CleanGrokLegacy() {
		cleaned = append(cleaned, "grok")
	}
	if CleanPiLegacy() {
		cleaned = append(cleaned, "pi")
	}
	if CleanCursorLegacy() {
		cleaned = append(cleaned, "cursor")
	}

	otherAgents := []struct {
		id   string
		path string
		key  string
	}{
		{"omp", filepath.Join(OmpAgentDirResolved(), "mcp.json"), "mcpServers.projectmem"},
		{"kilo", util.KiloPathsResolved().Config, "mcp.projectmem"},
		{"cline", util.ClinePathsResolved().McpConfig, "mcpServers.projectmem"},
		{"amp", util.AmpPathsResolved().Config, "amp.mcpServers.projectmem"},
		{"kimi", util.KimiPathsResolved().McpConfig, "mcpServers.projectmem"},
		{"zcode", util.ZcodePathsResolved().McpConfig, "mcp.servers.projectmem"},
		{"dsh", filepath.Join(util.Home(), ".dsh", "cordis.patch.yml"), "mcp_servers.projectmem"},
	}
	for _, oa := range otherAgents {
		if CleanGenericMcp(oa.path, oa.key) {
			cleaned = append(cleaned, oa.id)
		}
	}
	return cleaned
}

// ProbeAgentsLegacyCount checks how many agents still carry legacy artifacts.
func ProbeAgentsLegacyCount() int {
	count := 0
	if raw, ok := util.ReadFileSafe(util.ClaudeCodePaths().GlobalJSON); ok && strings.Contains(raw, "projectmem") {
		count++
	}
	if raw, ok := util.ReadFileSafe(codexHooksFile()); ok && strings.Contains(raw, "projectmem-hook") {
		count++
	} else if raw, ok := util.ReadFileSafe(util.CodexPathsResolved().Config); ok && strings.Contains(raw, "mcp_servers.projectmem") {
		count++
	}
	if raw, ok := util.ReadFileSafe(antigravityHooksFile()); ok && (strings.Contains(raw, "projectmem-context") || strings.Contains(raw, "tokless-projectmem")) {
		count++
	}
	if util.Exists(copilotHooksFile("projectmem-context.json")) || util.Exists(copilotHooksFile("tokless-projectmem.json")) {
		count++
	}
	if raw, ok := util.ReadFileSafe(droidRawHooks()); ok && strings.Contains(raw, "projectmem-hook") {
		count++
	}
	if raw, ok := util.ReadFileSafe(util.OpenCodePathsResolved().Config); ok && (strings.Contains(raw, "caveman") || strings.Contains(raw, "projectmem")) {
		count++
	}
	if PiSourceHas("npm:pi-caveman") || PiSourceHas("npm:pi-ponytail") {
		count++
	}
	return count
}
