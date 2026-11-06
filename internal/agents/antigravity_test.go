package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/util"
)

func TestConfigureAntigravityMcp_AdoptBareCodegraph(t *testing.T) {
	withMcpTest(t)
	mcpFile := filepath.Join(util.Home(), ".gemini", "config", "mcp_config.json")
	if err := util.EnsureDir(filepath.Dir(mcpFile)); err != nil {
		t.Fatal(err)
	}

	// 1. Seed bare codegraph entry as written by official `codegraph install`
	bareEntry := `{
  "mcpServers": {
    "codegraph": {
      "command": "codegraph",
      "args": ["serve", "--mcp"]
    }
  }
}`
	if err := os.WriteFile(mcpFile, []byte(bareEntry), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. ConfigureAntigravityMcp should adopt and upgrade it
	changed, file := ConfigureAntigravityMcp("codegraph")
	if !changed {
		t.Errorf("expected ConfigureAntigravityMcp to change bare entry, got changed=false (file=%s)", file)
	}

	// 3. Verify it is now bounded
	if !AntigravityMcpBounded("codegraph") {
		t.Errorf("expected AntigravityMcpBounded to be true after adoption")
	}

	// 4. Verify content has trust: true
	raw, ok := util.ReadFileSafe(mcpFile)
	if !ok || !strings.Contains(raw, `"trust": true`) {
		t.Errorf("expected mcp config to have trust: true, got:\n%s", raw)
	}
}

func TestConfigureAntigravityMcp_ProtectForeignEntry(t *testing.T) {
	withMcpTest(t)
	foreignEntry := `{
  "mcpServers": {
    "codegraph": {
      "command": "python",
      "args": ["my_script.py"]
    }
  }
}`
	for _, f := range antigravityMcpConfigFiles() {
		if err := util.EnsureDir(filepath.Dir(f)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(foreignEntry), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 2. ConfigureAntigravityMcp should refuse to overwrite foreign entry
	changed, _ := ConfigureAntigravityMcp("codegraph")
	if changed {
		t.Errorf("expected foreign entry not to be overwritten")
	}

	// 3. Verify it is not bounded
	if AntigravityMcpBounded("codegraph") {
		t.Errorf("foreign entry should not be considered bounded")
	}

	// 4. Verify content is unchanged in all files
	for _, f := range antigravityMcpConfigFiles() {
		raw, _ := util.ReadFileSafe(f)
		if !strings.Contains(raw, "my_script.py") {
			t.Errorf("foreign entry was modified in %s: %s", f, raw)
		}
	}
}

func TestRemoveAntigravityMcp_CleansAdoptableAndLegacy(t *testing.T) {
	withMcpTest(t)
	// Seed adoptable in active files and legacy files
	legacyFiles := antigravityLegacyMcpFiles()
	activeFiles := antigravityMcpConfigFiles()
	allFiles := append(activeFiles, legacyFiles...)

	adoptableEntry := `{
  "mcpServers": {
    "codegraph": {
      "command": "codegraph",
      "args": ["serve", "--mcp"]
    }
  }
}`
	for _, f := range allFiles {
		if err := util.EnsureDir(filepath.Dir(f)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(adoptableEntry), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	RemoveAntigravityMcp("codegraph")

	for _, f := range allFiles {
		raw, ok := util.ReadFileSafe(f)
		if ok && strings.Contains(raw, `"codegraph"`) {
			t.Errorf("expected codegraph to be removed from %s, got:\n%s", f, raw)
		}
	}
}
