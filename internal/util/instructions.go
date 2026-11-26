package util

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToklessOwners is render order: discipline skills then tools.
var ToklessOwners = []string{
	"principles",
	"caveman",
	"ponytail",
	"codegraph",
	"context-mode",
}

// SectionsByOwner maps each owner to its heading marker.
var SectionsByOwner = map[string]string{
	"principles":   "## Surgical Coding (principles)",
	"caveman":      "## Terse Output (caveman)",
	"ponytail":     "## Minimal Diff (ponytail)",
	"codegraph":    "## Code Index (codegraph)",
	"context-mode": "## Context Tools (context-mode)",
}

// CursorProjectRuleSpec describes one checked-in Cursor project rule.
type CursorProjectRuleSpec struct {
	Filename    string
	Description string
	Owner       string
}

var cursorProjectRuleSpecs = []CursorProjectRuleSpec{
	{Filename: "code-index.mdc", Description: "This rule guides codebase exploration through relevant symbols, flows, dependencies, and affected areas.", Owner: "codegraph"},
	{Filename: "context-tools.mdc", Description: "This rule guides efficient context collection, analysis, and retrieval while keeping raw data focused.", Owner: "context-mode"},
	{Filename: "surgical-coding.mdc", Description: "This rule keeps edits surgical, clarifies ambiguous requests, and uses existing project verification without unrequested test files.", Owner: "principles"},
	{Filename: "terse-output.mdc", Description: "This rule compresses user-facing prose while keeping code, commands, paths, and reasoning intact.", Owner: "caveman"},
	{Filename: "minimal-diff.mdc", Description: "This rule prioritizes reusing existing project helpers and standard library before adding abstractions.", Owner: "ponytail"},
}

// CursorProjectRuleContent renders one Cursor project rule from embedded instructions.
func CursorProjectRuleContent(spec CursorProjectRuleSpec) string {
	return "---\ndescription: " + spec.Description + "\nalwaysApply: true\n---\n" + instructionSection(spec.Owner) + "\n"
}

// CursorProjectRuleSpecs returns the Cursor project rule specifications.
func CursorProjectRuleSpecs() []CursorProjectRuleSpec {
	return append([]CursorProjectRuleSpec(nil), cursorProjectRuleSpecs...)
}

// IsToklessCursorRuleContent reports whether raw matches a Tokless-managed Cursor rule file.
func IsToklessCursorRuleContent(raw string, spec CursorProjectRuleSpec) bool {
	prefix := "---\ndescription: " + spec.Description + "\nalwaysApply: true\n---\n" + SectionsByOwner[spec.Owner]
	return strings.HasPrefix(raw, prefix)
}

// InstallCursorProjectRules installs or updates Tokless-managed Cursor rules in workspace,
// never overwriting user-authored files, and removing generated files when their tool is deselected.
func InstallCursorProjectRules(workspace string, dryRun bool) (bool, error) {
	if workspace == "" {
		return false, fmt.Errorf("Cursor project rules require non-empty workspace")
	}
	info, err := os.Stat(workspace)
	if err != nil {
		return false, fmt.Errorf("stat Cursor workspace: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("Cursor workspace is not a directory: %s", workspace)
	}
	if dryRun {
		return true, nil
	}
	rulesDir := filepath.Join(workspace, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return false, fmt.Errorf("create Cursor rules directory: %w", err)
	}
	for _, spec := range cursorProjectRuleSpecs {
		path := filepath.Join(rulesDir, spec.Filename)
		if !IsToolSelected(spec.Owner) {
			if data, readErr := os.ReadFile(path); readErr == nil && IsToklessCursorRuleContent(string(data), spec) {
				_ = os.Remove(path)
			}
			continue
		}
		want := CursorProjectRuleContent(spec)
		if existing, readErr := os.ReadFile(path); readErr == nil {
			if string(existing) == want || !IsToklessCursorRuleContent(string(existing), spec) {
				continue
			}
		}
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			return false, fmt.Errorf("write Cursor rule %s: %w", spec.Filename, err)
		}
	}
	return true, nil
}

var legacySectionsByOwner = map[string][]string{
	"principles":   {"## 1. Principles", "## Principles", "## Principles (craft)"},
	"caveman":      {"## 2. Response Style", "## Response Style (caveman)", "## Voice (caveman)"},
	"ponytail":     {"## 3. Build Discipline", "## Build Discipline (ponytail)", "## Reuse Ladder (ponytail)", "## Lazy Ladder (ponytail)"},
	"codegraph":    {"## 4. Code Search", "## Codegraph", "## Code Index (codegraph)"},
	"context-mode": {"## 5. Context Control", "## Context Tools", "## Context Tools (context-mode)"},
}

func SectionPresent(body, owner string) bool {
	for _, marker := range SectionMarkers(owner) {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func SectionMarkers(owner string) []string {
	marker, ok := SectionsByOwner[owner]
	if !ok {
		return nil
	}
	markers := []string{marker}
	markers = append(markers, legacySectionsByOwner[owner]...)
	return markers
}

// StripSection removes the given owner's section from body.
func StripSection(body string, owner string) string {
	heading, ok := SectionsByOwner[owner]
	if !ok {
		return body
	}
	return stripHeading(body, heading)
}

func stripHeading(body, heading string) string {
	idx := strings.Index(body, heading)
	if idx < 0 {
		return body
	}
	before := strings.TrimRight(body[:idx], "\n")
	after := body[idx+len(heading):]
	if next := strings.Index(after, "\n## "); next >= 0 {
		after = after[next+1:]
	} else {
		after = ""
	}
	after = strings.TrimLeft(after, "\n")
	if before == "" {
		return after
	}
	if after == "" {
		return before
	}
	return before + "\n\n" + after
}

//go:embed agent_instructions.md
var agentInstructionsTemplate string

// instructionIndexSection returns the index / summary section at the top.
func instructionIndexSection() string {
	raw := normalizedAgentInstructions()
	idx := strings.Index(raw, "\n## ")
	if idx < 0 {
		return strings.TrimRight(raw, "\n")
	}
	return strings.TrimRight(raw[:idx], "\n")
}

// instructionSection extracts the section for owner from agent_instructions.md.
func instructionSection(owner string) string {
	heading, ok := SectionsByOwner[owner]
	if !ok {
		return ""
	}
	raw := normalizedAgentInstructions()
	idx := strings.Index(raw, heading)
	if idx < 0 {
		return ""
	}
	rest := raw[idx:]
	if idx := strings.Index(rest[1:], "\n## "); idx >= 0 {
		return strings.TrimRight(rest[:idx+1], "\n")
	}
	return strings.TrimRight(rest, "\n")
}

func normalizedAgentInstructions() string {
	return strings.ReplaceAll(agentInstructionsTemplate, "\r\n", "\n")
}

// ToklessAgentBody renders the full markdown body for the given owners.
func ToklessAgentBody(owners []string) string {
	var b strings.Builder

	if len(owners) >= 2 {
		b.WriteString(instructionIndexSection())
		b.WriteString("\n\n")
	}
	for _, owner := range ToklessOwners {
		if hasOwner(owners, owner) {
			b.WriteString(instructionSection(owner))
			b.WriteString("\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// TokenizeBody infers active owners from section headings present in body.
func TokenizeBody(body string) []string {
	var out []string
	for _, owner := range ToklessOwners {
		if SectionPresent(body, owner) {
			out = append(out, owner)
		}
	}
	return out
}

func hasOwner(owners []string, want string) bool {
	for _, o := range owners {
		if o == want {
			return true
		}
	}
	return false
}
