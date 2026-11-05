package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/core"
	toolsPkg "github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

// LegacyCursorRules is the list of legacy rule filenames created by original Tokless.
var LegacyCursorRules = []string{
	"principles.mdc",
	"response-style.mdc",
	"build-discipline.mdc",
	"project-memory.mdc",
	"caveman.mdc",
	"ponytail.mdc",
}

// CleanWorkspaceCursorRules removes legacy rule files from workspace and global .cursor/rules dirs.
func CleanWorkspaceCursorRules() []string {
	var cleaned []string
	roots := []string{"."}
	if home := util.Home(); home != "" {
		roots = append(roots, home)
	}
	if p, err := filepath.Abs("."); err == nil {
		roots = append(roots, p)
	}
	seen := map[string]bool{}
	for _, root := range roots {
		rulesDir := filepath.Join(root, ".cursor", "rules")
		if seen[rulesDir] || !util.Exists(rulesDir) {
			continue
		}
		seen[rulesDir] = true
		for _, name := range LegacyCursorRules {
			p := filepath.Join(rulesDir, name)
			if util.Exists(p) {
				if err := os.Remove(p); err == nil {
					cleaned = append(cleaned, p)
				}
			}
		}
		if entries, err := os.ReadDir(rulesDir); err == nil && len(entries) == 0 {
			_ = os.Remove(rulesDir)
		}
	}
	return cleaned
}

// CleanAllLegacyInstructions strips legacy instructions from all registered agent instruction files.
func CleanAllLegacyInstructions() []string {
	var cleaned []string
	for _, agent := range core.ListAgents() {
		p := toolsPkg.InstructionPath(agent.ID)
		if p == "" || !util.Exists(p) {
			continue
		}
		raw, ok := util.ReadFileSafe(p)
		if !ok {
			continue
		}
		stripped := toolsPkg.StripLegacyExported(raw)
		if stripped != raw {
			if strings.TrimSpace(stripped) == "" {
				_ = os.Remove(p)
			} else {
				_ = util.WriteFile(p, strings.TrimRight(stripped, "\n")+"\n")
			}
			cleaned = append(cleaned, agent.ID)
		}
	}
	return cleaned
}

// CleanGlobalNpmPackages uninstalls legacy projectmem npm package if installed.
func CleanGlobalNpmPackages() []string {
	if os.Getenv("TOKLESS_TEST") == "1" {
		return nil
	}
	var cleaned []string
	npm := util.ResolveNpmBinary()
	if npm == "" {
		return cleaned
	}
	for _, pkg := range []string{"projectmem"} {
		if util.NpmInstalledVersionExported(pkg) != nil {
			r := util.Run(npm, []string{"uninstall", "-g", pkg}, util.RunOptions{Capture: true})
			if r.Code == 0 {
				cleaned = append(cleaned, pkg)
			}
		}
	}
	return cleaned
}

// CleanLegacyArtifacts cleans all legacy original Tokless artifacts.
func CleanLegacyArtifacts(dryRun bool) (cleaned []string, errs []error) {
	if dryRun {
		count := ProbeLegacyArtifactsCount()
		if count > 0 {
			util.L.Sub(fmt.Sprintf("[dry-run] would clean %d legacy Tokless artifact(s)", count))
		}
		return nil, nil
	}
	agentCleaned := agents.CleanAllAgentsLegacy()
	cleaned = append(cleaned, agentCleaned...)

	instCleaned := CleanAllLegacyInstructions()
	if len(instCleaned) > 0 {
		cleaned = append(cleaned, "instructions:"+strings.Join(instCleaned, ","))
	}

	rulesCleaned := CleanWorkspaceCursorRules()
	if len(rulesCleaned) > 0 {
		cleaned = append(cleaned, "cursor-rules:"+strings.Join(rulesCleaned, ","))
	}

	npmCleaned := CleanGlobalNpmPackages()
	if len(npmCleaned) > 0 {
		cleaned = append(cleaned, "npm:"+strings.Join(npmCleaned, ","))
	}

	return cleaned, errs
}

// ProbeLegacyArtifactsCount reports whether any legacy artifacts remain.
func ProbeLegacyArtifactsCount() int {
	count := agents.ProbeAgentsLegacyCount()
	for _, name := range LegacyCursorRules {
		if util.Exists(filepath.Join(".cursor", "rules", name)) {
			count++
		}
	}
	for _, agent := range core.ListAgents() {
		p := toolsPkg.InstructionPath(agent.ID)
		if p != "" && util.Exists(p) {
			if raw, ok := util.ReadFileSafe(p); ok {
				if toolsPkg.StripLegacyExported(raw) != raw {
					count++
				}
			}
		}
	}
	if os.Getenv("TOKLESS_TEST") != "1" && util.NpmInstalledVersionExported("projectmem") != nil {
		count++
	}
	return count
}
