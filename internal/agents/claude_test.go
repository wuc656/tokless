package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/util"
)

func TestAllowClaudeMcpToolProjectLocalPreservesAndAppends(t *testing.T) {
	proj := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	settings := filepath.Join(proj, ".claude", "settings.local.json")
	seed := `{
  "permissions": {
    "allow": [
      "WebSearch",
      "mcp__context-mode__ctx_search"
    ]
  }
}
`
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	AllowClaudeMcpToolProjectLocal("context-mode")
	AllowClaudeMcpToolProjectLocal("codegraph")
	AllowClaudeMcpToolProjectLocal("context-mode")

	raw, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"WebSearch",
		"mcp__context-mode__ctx_search",
		"mcp__context-mode__ctx_execute",
		"mcp__context-mode__ctx_execute_file",
		"mcp__context-mode__ctx_batch_execute",
		"mcp__context-mode__ctx_batch",
		"mcp__context-mode__ctx_index",
		"mcp__context-mode__ctx_fetch_and_index",
		"mcp__codegraph__.*",
		"mcp__codegraph__codegraph_explore",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
		if strings.Count(got, `"`+want+`"`) != 1 {
			t.Fatalf("duplicate %q in %s", want, got)
		}
	}
	if strings.Contains(got, "mcp__context-mode__.*") {
		t.Fatalf("context-mode wildcard should be migrated: %s", got)
	}
}

func TestAllowClaudeMcpToolProjectLocalCreatesFile(t *testing.T) {
	proj := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(proj); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	AllowClaudeMcpToolProjectLocal("codegraph")

	raw, err := os.ReadFile(filepath.Join(proj, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "mcp__codegraph__.*") || !strings.Contains(got, "mcp__codegraph__codegraph_explore") {
		t.Fatalf("missing codegraph permissions in %s", got)
	}
}

func TestConfigureClaudeMcp_RefusesForeignEntry(t *testing.T) {
	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })

	p := util.ClaudeCodePaths()
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	foreignJSON := `{
  "mcpServers": {
    "context-mode": {
      "type": "stdio",
      "command": "my-wrapper",
      "args": ["context-mode"]
    }
  }
}`
	if err := os.WriteFile(p.GlobalJSON, []byte(foreignJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, file := ConfigureClaudeMcp("context-mode")
	if changed {
		t.Fatalf("expected ConfigureClaudeMcp to return changed=false for foreign entry, got changed=true (file=%s)", file)
	}

	raw, err := os.ReadFile(p.GlobalJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "my-wrapper") {
		t.Fatalf("foreign entry was modified: %s", string(raw))
	}
}
