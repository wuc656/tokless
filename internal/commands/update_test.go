package commands

import (
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/tools"
	"github.com/HoangP8/tokless/internal/util"
)

func TestUpdateToolFlowSelectionAndInstallation(t *testing.T) {
	for _, id := range []string{"headroom"} {
		needsInstall, _, _ := updateToolFlow(id, util.VersionInfo{}, nil)
		if !needsInstall {
			t.Errorf("%s missing install despite unknown latest version", id)
		}

		called := false
		_, verified, err := updateToolFlow(id, util.VersionInfo{}, func() (bool, bool, error) {
			called = true
			return true, false, nil
		})
		if !called || err != nil || verified {
			t.Errorf("%s accepted installer success without package presence", id)
		}

		_, verified, err = updateToolFlow(id, util.VersionInfo{}, func() (bool, bool, error) {
			return true, true, nil
		})
		if err != nil || !verified {
			t.Errorf("%s rejected installer success with package presence", id)
		}
	}
}

func legacyCtxMcpConfig(t *testing.T) string {
	t.Helper()
	tools.WriteOwner("opencode", "context-mode")
	cfgPath := util.OpenCodePathsResolved().Config
	spawn := util.PickMcpSpawn("context-mode")
	body := `{"mcp":{"context-mode":{"type":"local","command":["` + spawn.Command + `","run-mcp","--context-mode"],"enabled":true}}}`
	if err := util.WriteFile(cfgPath, body); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestRunUpdateToolsFlagScopesDryRun(t *testing.T) {
	openCodeUpdateEnv(t)
	legacyCtxMcpConfig(t)
	all := []string{"rtk", "codegraph", "context-mode", "headroom"}
	for _, tc := range []struct {
		name  string
		tools []string
		rows  []string
	}{
		{"context-mode only", []string{"context-mode"}, []string{"context-mode"}},
		{"codegraph only", []string{"codegraph"}, []string{"codegraph"}},
		{"two tools", []string{"context-mode", "headroom"}, []string{"context-mode", "headroom"}},
		{"no flag lists every tool", nil, all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() { RunUpdate(InitOptions{DryRun: true, Tools: tc.tools}) })
			for _, id := range all {
				// Version rows pad the ID to 14 columns; the upgrade summary does not.
				want := contains(tc.rows, id)
				if got := strings.Contains(out, padEnd(id, 14)); got != want {
					t.Errorf("row %s listed = %v, want %v:\n%s", id, got, want, out)
				}
			}
		})
	}
	out := captureStdout(t, func() { RunUpdate(InitOptions{DryRun: true, Tools: []string{"context-mode"}}) })
	if !strings.Contains(out, "Would upgrade: context-mode") || strings.Contains(out, "codegraph") {
		t.Fatalf("scoped dry run must only upgrade context-mode:\n%s", out)
	}
}

func TestRunUpdateToolsFlagUnknownID(t *testing.T) {
	openCodeUpdateEnv(t)
	for _, ids := range [][]string{{"nope"}, {"context-mode", "nope"}} {
		var code int
		out := captureStdout(t, func() { code = RunUpdate(InitOptions{DryRun: true, Tools: ids}) })
		if code == 0 || strings.Contains(out, "Versions") {
			t.Fatalf("%v: code = %d, want non-zero before any probe:\n%s", ids, code, out)
		}
	}
}

func TestRunUpdateToolsFlagLeavesOtherToolsUntouched(t *testing.T) {
	openCodeUpdateEnv(t)
	cfgPath := legacyCtxMcpConfig(t)
	captureStdout(t, func() { RunUpdate(InitOptions{Yes: true, Tools: []string{"context-mode"}}) })

	if raw, _ := util.ReadFileSafe(cfgPath); strings.Contains(raw, "run-mcp") || !strings.Contains(raw, "context-mode@") {
		t.Fatalf("context-mode MCP entry not migrated to plugin:\n%s", raw)
	}
}

func TestResyncInstructionWiringScopedSkipsUnlisted(t *testing.T) {
	openCodeUpdateEnv(t)
	instrPath := util.OpenCodePathsResolved().Instructions
	const stale = "# notes\n\n## Response Style (caveman)\nstale\n"
	write(t, instrPath, stale)

	resyncInstructionWiring("context-mode")
	if raw, _ := util.ReadFileSafe(instrPath); raw != stale {
		t.Fatalf("unlisted instruction tools were resynced:\n%s", raw)
	}
	resyncInstructionWiring()
	if raw, _ := util.ReadFileSafe(instrPath); strings.Contains(raw, "stale") {
		t.Fatalf("unscoped resync must refresh the stale body:\n%s", raw)
	}
}

func TestResyncInstructionWiringUpgradesFromV1_0_42(t *testing.T) {
	agents.Register()
	tools.Register()
	t.Setenv("TOKLESS_TEST", "1")

	t.Run("full mode wires new instruction tools and refreshes existing sections", func(t *testing.T) {
		withSelHome(t)
		seedAgentInstalled(t, "claude")
		if err := util.SaveSelection("full", []string{}); err != nil {
			t.Fatal(err)
		}
		instrPath := util.ClaudeCodePaths().Instructions
		const v142 = "# User Notes\n\n## Code Index (codegraph)\n- old git ls-files | wc -l\n\n## Context Tools (context-mode)\n- old verbose block\n"
		write(t, instrPath, v142)

		resyncInstructionWiring()

		raw, ok := util.ReadFileSafe(instrPath)
		if !ok {
			t.Fatalf("missing instructions file %s", instrPath)
		}
		if !strings.Contains(raw, "# User Notes") {
			t.Fatalf("user content was not preserved:\n%s", raw)
		}
		if strings.Contains(raw, "git ls-files") || strings.Contains(raw, "old verbose block") {
			t.Fatalf("stale v1.0.42 template text remained:\n%s", raw)
		}
		for _, want := range []string{
			"## Surgical Coding (principles)",
			"## Terse Output (caveman)",
			"## Minimal Diff (ponytail)",
			"## Code Index (codegraph)",
			"## Context Tools (context-mode)",
		} {
			if !strings.Contains(raw, want) {
				t.Fatalf("missing upgraded section %q:\n%s", want, raw)
			}
		}
	})

	t.Run("selected mode refreshes chosen section without adding unselected tools", func(t *testing.T) {
		withSelHome(t)
		seedAgentInstalled(t, "claude")
		if err := util.SaveSelection("selected", []string{"codegraph"}); err != nil {
			t.Fatal(err)
		}
		instrPath := util.ClaudeCodePaths().Instructions
		write(t, instrPath, "## Code Index (codegraph)\n- old git ls-files | wc -l\n")

		resyncInstructionWiring()

		raw, ok := util.ReadFileSafe(instrPath)
		if !ok {
			t.Fatalf("missing instructions file %s", instrPath)
		}
		if strings.Contains(raw, "git ls-files") || !strings.Contains(raw, "codegraph_explore") {
			t.Fatalf("selected codegraph section was not refreshed:\n%s", raw)
		}
		for _, unwanted := range []string{
			"## Surgical Coding (principles)",
			"## Terse Output (caveman)",
			"## Minimal Diff (ponytail)",
		} {
			if strings.Contains(raw, unwanted) {
				t.Fatalf("unselected section %q was added:\n%s", unwanted, raw)
			}
		}
	})
}
