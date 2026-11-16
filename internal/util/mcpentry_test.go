package util

import (
	"testing"
)

func TestEqualPaths(t *testing.T) {
	if !EqualPaths("C:\\foo\\bar.cmd", "c:/foo/bar.cmd") {
		t.Errorf("expected EqualPaths to be true for slash and case differences")
	}
	if !EqualPaths("C:\\foo\\bar.CMD", "C:\\foo\\bar.cmd") {
		t.Errorf("expected EqualPaths to be true for .CMD vs .cmd")
	}
	if EqualPaths("C:\\foo\\bar.cmd", "C:\\foo\\baz.cmd") {
		t.Errorf("expected EqualPaths to be false for different files")
	}
}

func TestCommandMatches(t *testing.T) {
	if !CommandMatches("tokless", "tokless.exe") {
		t.Errorf("expected CommandMatches to be true for tokless vs tokless.exe")
	}
	if !CommandMatches("C:\\Users\\bin\\tokless.exe", "tokless") {
		t.Errorf("expected CommandMatches to be true for full path vs bare tokless")
	}
}

func TestArgsEqual(t *testing.T) {
	a := []any{"run-mcp", "--agent", "codex", "cmd", "/c", "C:\\tools\\codegraph.CMD", "serve", "--mcp"}
	b := []string{"run-mcp", "--agent", "codex", "cmd", "/c", "C:\\tools\\codegraph.cmd", "serve", "--mcp"}
	if !ArgsEqual(a, b) {
		t.Errorf("expected ArgsEqual to be true with .CMD vs .cmd in args")
	}
}

func TestIsToklessOrToolMcpEntry(t *testing.T) {
	m := NewOrderedMap()
	m.Set("command", "tokless.exe")
	m.Set("args", []any{"run-mcp", "--agent", "codex", "codegraph"})
	if !IsToklessOrToolMcpEntry("codegraph", m) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be true for tokless run-mcp codegraph")
	}

	upstream := NewOrderedMap()
	upstream.Set("command", "C:\\tools\\context-mode.cmd")
	upstream.Set("args", []any{})
	if !IsToklessOrToolMcpEntry("context-mode", upstream) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be true for upstream context-mode")
	}

	// Bare codegraph tool command
	bare := NewOrderedMap()
	bare.Set("command", "codegraph")
	bare.Set("args", []any{"serve", "--mcp"})
	if !IsToklessOrToolMcpEntry("codegraph", bare) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be true for bare codegraph serve --mcp")
	}

	// cmd.exe wrapper to codegraph.cmd
	cmdWrapper := NewOrderedMap()
	cmdWrapper.Set("command", "cmd.exe")
	cmdWrapper.Set("args", []any{"/c", "C:\\bin\\codegraph.cmd", "serve", "--mcp"})
	if !IsToklessOrToolMcpEntry("codegraph", cmdWrapper) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be true for cmd.exe wrapper")
	}

	// npx wrapper
	npxWrapper := NewOrderedMap()
	npxWrapper.Set("command", "npx")
	npxWrapper.Set("args", []any{"--no-install", "@colbymchenry/codegraph", "serve", "--mcp"})
	if !IsToklessOrToolMcpEntry("codegraph", npxWrapper) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be true for npx wrapper")
	}

	// Foreign wrapper tools with "codegraph" in args must be refused
	foreignWrapper := NewOrderedMap()
	foreignWrapper.Set("command", "my-wrapper")
	foreignWrapper.Set("args", []any{"codegraph", "serve", "--mcp"})
	if IsToklessOrToolMcpEntry("codegraph", foreignWrapper) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be false for arbitrary foreign wrapper tool")
	}

	pythonWrapper := NewOrderedMap()
	pythonWrapper.Set("command", "python")
	pythonWrapper.Set("args", []any{"run_codegraph.py"})
	if IsToklessOrToolMcpEntry("codegraph", pythonWrapper) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be false for python script")
	}

	unrelated := NewOrderedMap()
	unrelated.Set("command", "github-copilot-server")
	unrelated.Set("args", []any{})
	if IsToklessOrToolMcpEntry("codegraph", unrelated) {
		t.Errorf("expected IsToklessOrToolMcpEntry to be false for unrelated tool")
	}
}

func TestMcpEntryMatchesTool(t *testing.T) {
	m := NewOrderedMap()
	m.Set("command", "C:\\Users\\AppData\\tokless.exe")
	m.Set("args", []any{"run-mcp", "--context-mode", "cmd", "/c", "context-mode.CMD"})

	if !McpEntryMatchesTool("context-mode", "tokless", []string{"run-mcp", "--context-mode"}, m) {
		t.Errorf("expected McpEntryMatchesTool to return true")
	}
}
