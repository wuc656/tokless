package util

import (
	"strings"
	"testing"
)

func TestAgentInstructionsContent(t *testing.T) {
	owners := []string{"principles", "caveman", "ponytail", "codegraph", "context-mode"}
	body := ToklessAgentBody(owners)

	for _, section := range []string{
		"## Surgical Coding",
		"## Terse Output",
		"## Minimal Diff",
		"## Code Index (codegraph)",
		"## Context Tools (context-mode)",
	} {
		if !strings.Contains(body, section) {
			t.Errorf("MISSING section: %s", section)
		}
	}

	if strings.Contains(body, "hook") || strings.Contains(body, "Hook") {
		t.Error("instructions contain 'hook' — should be zero hooks")
	}
	for _, forbidden := range []string{
		"headroom_compress", "headroom_retrieve", "Headroom (headroom)",
		"test_*.py", "demo()", "__main__", "shortcut:", "start_session", "precheck_file", "record_attempt",
		"ask briefly", "Name a simpler option", "Trust results", "no re-read", "MANDATORY", "Do NOT answer",
		"git ls-files", "tokless index", "projectmem", "get_summary",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("instructions contain forbidden/high-interference rule: %s", forbidden)
		}
	}

	for _, forbidden := range []string{"ctx_execute_file", "ctx_batch_execute", "ctx_fetch_and_index", "ctx_insight"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("instructions contain removed context-mode 1.x tool: %s", forbidden)
		}
	}

	for _, kw := range []string{
		"codegraph_explore", "ctx_execute", "ctx_batch",
	} {
		if !strings.Contains(body, kw) {
			t.Errorf("MISSING keyword: %s", kw)
		}
	}
	t.Logf("body length: %d bytes", len(body))
}

func TestFullAgentBodyMatchesTemplate(t *testing.T) {
	if got, want := ToklessAgentBody(ToklessOwners)+"\n", normalizedAgentInstructions(); got != want {
		t.Fatal("full generated agent body drifted from embedded instructions template")
	}
}

func TestPerOwnerRendering(t *testing.T) {
	for _, tc := range []struct {
		owner   string
		marker  string
		example string
	}{
		{"principles", "## Surgical Coding", "Never create unrequested test files"},
		{"caveman", "## Terse Output", "Compress user-facing prose only"},
		{"ponytail", "## Minimal Diff", "Prefer existing project helpers"},
		{"codegraph", "## Code Index (codegraph)", "codegraph_explore"},
		{"context-mode", "## Context Tools (context-mode)", "ctx_execute"},
	} {
		body := ToklessAgentBody([]string{tc.owner})
		if !strings.Contains(body, tc.marker) {
			t.Errorf("%s: missing marker %q", tc.owner, tc.marker)
		}
		if !strings.Contains(body, tc.example) {
			t.Errorf("%s: missing example keyword %q", tc.owner, tc.example)
		}
	}
}
