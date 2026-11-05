package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/agents"
	toolsPkg "github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

func TestCleanLegacyArtifactsPurgesAllTraces(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Cleanup(func() { util.SetHomeOverride("") })

	// 1. Pre-seed Claude
	claudePath := util.ClaudeCodePaths().GlobalJSON
	_ = util.EnsureDir(filepath.Dir(claudePath))
	claudeCfg := `{"mcpServers":{"projectmem":{"command":"tokless","args":["run-mcp","--tool","projectmem"]},"codegraph":{"command":"codegraph"}},"permissions":{"allow":["mcp(projectmem/*)","mcp(codegraph/*)"]}}`
	if err := util.WriteFile(claudePath, claudeCfg); err != nil {
		t.Fatal(err)
	}

	// 2. Pre-seed Codex
	codexHooks := agents.CodexRtkHooksFile()
	_ = util.EnsureDir(filepath.Dir(codexHooks))
	codexHooksJSON := `{"hooks":{"SessionStart":[{"type":"command","command":"tokless projectmem-hook claude","timeout":15}]}}`
	if err := util.WriteFile(codexHooks, codexHooksJSON); err != nil {
		t.Fatal(err)
	}
	codexCfg := util.CodexPathsResolved().Config
	_ = util.WriteFile(codexCfg, "[mcp_servers.projectmem]\ncommand = \"tokless\"\nargs = [\"run-mcp\", \"--tool\", \"projectmem\"]\n\n[mcp_servers.codegraph]\ncommand = \"codegraph\"\n")

	// 3. Pre-seed Antigravity
	agyHooks := agents.AntigravityRtkHooksFile()
	_ = util.EnsureDir(filepath.Dir(agyHooks))
	agyHooksJSON := `{"projectmem-context":{"SessionStart":[{"type":"command","command":"tokless projectmem-hook agy"}]},"rtk-context":{"SessionStart":[]}}`
	_ = util.WriteFile(agyHooks, agyHooksJSON)
	agyMcp := util.AntigravityPathsResolved().McpConfig
	_ = util.EnsureDir(filepath.Dir(agyMcp))
	_ = util.WriteFile(agyMcp, `{"mcpServers":{"projectmem":{"command":"tokless"}},"permissions":{"allow":["mcp(projectmem/*)"]}}`)

	// 4. Pre-seed Copilot
	copilotRtk := agents.CopilotRtkHooksFile()
	copilotHooksDir := filepath.Dir(copilotRtk)
	_ = util.EnsureDir(copilotHooksDir)
	_ = util.WriteFile(filepath.Join(copilotHooksDir, "projectmem-context.json"), `{"version":1}`)
	_ = util.WriteFile(filepath.Join(copilotHooksDir, "tokless-projectmem.json"), `{"version":1}`)
	_ = util.WriteFile(util.CopilotPathsResolved().McpConfig, `{"mcpServers":{"projectmem":{"command":"tokless"}}}`)

	// 5. Pre-seed OpenCode
	ocDir := filepath.Dir(util.OpenCodePathsResolved().Config)
	_ = util.EnsureDir(ocDir)
	_ = util.WriteFile(util.OpenCodePathsResolved().Config, `{"plugin":["./plugins/caveman/plugin.js","other-plugin"],"mcp":{"projectmem":{"type":"local"}}}`)
	_ = util.EnsureDir(filepath.Join(ocDir, "plugins", "caveman"))
	_ = util.WriteFile(filepath.Join(ocDir, "plugins", "caveman", "plugin.js"), "module.exports = {}")
	_ = util.EnsureDir(filepath.Join(ocDir, "agents"))
	_ = util.WriteFile(filepath.Join(ocDir, "agents", "cavecrew-builder.md"), "# builder")
	_ = util.WriteFile(filepath.Join(ocDir, "agents", "user-custom.md"), "# custom")

	// 6. Pre-seed Pi
	piMcp := filepath.Join(agents.PiAgentDirResolved(), "mcp.json")
	_ = util.EnsureDir(filepath.Dir(piMcp))
	_ = util.WriteFile(piMcp, `{"mcpServers":{"projectmem":{"command":"tokless"}}}`)
	piSkills := agents.PiSkillsDir()
	_ = util.EnsureDir(filepath.Join(piSkills, "caveman"))
	_ = util.WriteFile(filepath.Join(piSkills, "caveman", "SKILL.md"), "# caveman")
	_ = util.EnsureDir(filepath.Join(piSkills, "ponytail"))
	_ = util.WriteFile(filepath.Join(piSkills, "ponytail", "SKILL.md"), "# ponytail")

	// 7. Pre-seed Instruction Files with legacy headings
	claudeInst := toolsPkg.InstructionPath("claude")
	_ = util.EnsureDir(filepath.Dir(claudeInst))
	instBody := `# Agent Instructions

Apply on every coding task:

**Skills (apply):**
- Principles — think, simplify, edit surgically, verify.
- Response Style (caveman) — terse prose, full technical accuracy.
- Build Discipline (ponytail) — reuse first, write only what must exist.

**Tools (call):**
- Code Index (codegraph) — MCP codegraph_explore for structure.
- Context Tools (context-mode) — MCP ctx_* for analysis.
- Project Memory (projectmem) — MCP tools log issues.

## Principles
Some principles content.

## Response Style (caveman)
Terse prose.

## Build Discipline (ponytail)
Reuse first.

## Code Index (codegraph)
Real codegraph docs.

## Project Memory (projectmem)
Memory docs.
`
	_ = util.WriteFile(claudeInst, instBody)

	// 8. Pre-seed Cursor rules
	workspace := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(workspace)
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	rulesDir := filepath.Join(workspace, ".cursor", "rules")
	_ = util.EnsureDir(rulesDir)
	for _, rule := range []string{"principles.mdc", "response-style.mdc", "build-discipline.mdc", "project-memory.mdc", "code-index.mdc"} {
		_ = util.WriteFile(filepath.Join(rulesDir, rule), "---\nalwaysApply: true\n---\n")
	}

	// Verify legacy items detected before cleanup
	probeCount := ProbeLegacyArtifactsCount()
	if probeCount == 0 {
		t.Fatal("expected legacy artifacts to be detected before clean")
	}

	// RUN CLEANUP
	cleaned, errs := CleanLegacyArtifacts(false)
	if len(errs) > 0 {
		t.Fatalf("unexpected cleanup errors: %v", errs)
	}
	if len(cleaned) == 0 {
		t.Fatal("expected cleaned items, got 0")
	}

	// VERIFY PURGED:
	// 1. Claude
	raw, _ := util.ReadFileSafe(claudePath)
	if strings.Contains(raw, "projectmem") {
		t.Fatalf("Claude still contains projectmem: %s", raw)
	}
	if !strings.Contains(raw, "codegraph") {
		t.Fatalf("Claude lost active codegraph MCP: %s", raw)
	}

	// 2. Codex
	raw, _ = util.ReadFileSafe(codexHooks)
	if strings.Contains(raw, "projectmem-hook") {
		t.Fatalf("Codex hooks still contains projectmem-hook: %s", raw)
	}
	raw, _ = util.ReadFileSafe(codexCfg)
	if strings.Contains(raw, "projectmem") {
		t.Fatalf("Codex config.toml still contains projectmem: %s", raw)
	}
	if !strings.Contains(raw, "codegraph") {
		t.Fatalf("Codex config.toml lost active codegraph MCP: %s", raw)
	}

	// 3. Antigravity
	raw, _ = util.ReadFileSafe(agyHooks)
	if strings.Contains(raw, "projectmem-context") {
		t.Fatalf("Antigravity hooks still contains projectmem-context: %s", raw)
	}
	raw, _ = util.ReadFileSafe(agyMcp)
	if strings.Contains(raw, "projectmem") {
		t.Fatalf("Antigravity MCP config still contains projectmem: %s", raw)
	}

	// 4. Copilot
	if util.Exists(filepath.Join(copilotHooksDir, "projectmem-context.json")) {
		t.Fatal("Copilot projectmem-context.json was not deleted")
	}
	if util.Exists(filepath.Join(copilotHooksDir, "tokless-projectmem.json")) {
		t.Fatal("Copilot tokless-projectmem.json was not deleted")
	}
	raw, _ = util.ReadFileSafe(util.CopilotPathsResolved().McpConfig)
	if strings.Contains(raw, "projectmem") {
		t.Fatalf("Copilot MCP still contains projectmem: %s", raw)
	}

	// 5. OpenCode
	raw, _ = util.ReadFileSafe(util.OpenCodePathsResolved().Config)
	if strings.Contains(raw, "caveman") || strings.Contains(raw, "projectmem") {
		t.Fatalf("OpenCode config still contains legacy artifacts: %s", raw)
	}
	if !strings.Contains(raw, "other-plugin") {
		t.Fatalf("OpenCode lost user plugin: %s", raw)
	}
	if util.Exists(filepath.Join(ocDir, "plugins", "caveman")) {
		t.Fatal("OpenCode caveman plugin dir still exists")
	}
	if util.Exists(filepath.Join(ocDir, "agents", "cavecrew-builder.md")) {
		t.Fatal("OpenCode cavecrew-builder.md still exists")
	}
	if !util.Exists(filepath.Join(ocDir, "agents", "user-custom.md")) {
		t.Fatal("OpenCode user custom agent file was mistakenly deleted")
	}

	// 6. Pi
	raw, _ = util.ReadFileSafe(piMcp)
	if strings.Contains(raw, "projectmem") {
		t.Fatalf("Pi MCP still contains projectmem: %s", raw)
	}
	if util.Exists(filepath.Join(piSkills, "caveman")) {
		t.Fatal("Pi caveman skill dir still exists")
	}
	if util.Exists(filepath.Join(piSkills, "ponytail")) {
		t.Fatal("Pi ponytail skill dir still exists")
	}

	// 7. Instruction files
	raw, _ = util.ReadFileSafe(claudeInst)
	if strings.Contains(raw, "## Principles") || strings.Contains(raw, "## Response Style (caveman)") ||
		strings.Contains(raw, "## Build Discipline (ponytail)") || strings.Contains(raw, "## Project Memory (projectmem)") {
		t.Fatalf("Claude instructions still contain legacy sections: %s", raw)
	}

	// 8. Cursor rules
	for _, rule := range []string{"principles.mdc", "response-style.mdc", "build-discipline.mdc", "project-memory.mdc"} {
		if util.Exists(filepath.Join(rulesDir, rule)) {
			t.Fatalf("Cursor legacy rule %s was not deleted", rule)
		}
	}
	if !util.Exists(filepath.Join(rulesDir, "code-index.mdc")) {
		t.Fatal("Cursor active rule code-index.mdc was mistakenly deleted")
	}

	// IDEMPOTENCY TEST: Running again produces 0 errors and no-op
	cleanedSecond, errsSecond := CleanLegacyArtifacts(false)
	if len(errsSecond) > 0 {
		t.Fatalf("second cleanup had errors: %v", errsSecond)
	}
	if len(cleanedSecond) > 0 {
		t.Fatalf("second cleanup was not idempotent: cleaned %v", cleanedSecond)
	}
}

func TestCleanLegacyArtifactsDryRunDoesNotMutate(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })

	claudePath := util.ClaudeCodePaths().GlobalJSON
	_ = util.EnsureDir(filepath.Dir(claudePath))
	claudeCfg := `{"mcpServers":{"projectmem":{"command":"tokless"}}}`
	_ = util.WriteFile(claudePath, claudeCfg)

	cleaned, errs := CleanLegacyArtifacts(true)
	if len(errs) > 0 || len(cleaned) > 0 {
		t.Fatalf("dry run unexpected output: cleaned=%v, errs=%v", cleaned, errs)
	}
	raw, _ := util.ReadFileSafe(claudePath)
	if raw != claudeCfg {
		t.Fatalf("dry run modified file: %s", raw)
	}
}
