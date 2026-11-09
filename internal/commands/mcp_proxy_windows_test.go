//go:build windows

package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/util"
)

func TestCmdBatchRelayWithForwardSlashPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "relay fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "relay fixture.cmd")
	if err := os.WriteFile(fixture, []byte("@echo relay-ok\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	_, args := resolveMcpCommand(comspec, []string{comspec, "/c", filepath.ToSlash(fixture)})
	out, err := exec.Command(comspec, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("cmd relay failed: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "relay-ok") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestResolveWindowsBatchToNodeWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Author Software", "nvm", "installs", "v26.8.1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsDir := filepath.Join(dir, "node_modules", "@colbymchenry", "codegraph")
	if err := os.MkdirAll(jsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsFile := filepath.Join(jsDir, "npm-shim.js")
	if err := os.WriteFile(jsFile, []byte("console.log('shim-executed-ok');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batchContent := `@ECHO off
GOTO start
:find_dp0
SET dp0=%~dp0
EXIT /b
:start
SETLOCAL
CALL :find_dp0

endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & "%_prog%"  "%dp0%\node_modules\@colbymchenry\codegraph\npm-shim.js" %*
`
	batchFile := filepath.Join(dir, "codegraph.cmd")
	if err := os.WriteFile(batchFile, []byte(batchContent), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(batchFile)
	if err != nil {
		t.Fatal(err)
	}
	m := reNpmBatchTarget.FindSubmatch(content)
	if m == nil {
		t.Fatalf("regex failed to match batch content: %q", string(content))
	}

	_ = util.Exists
	relJS := string(m[1])
	relJS = strings.ReplaceAll(relJS, "%dp0%\\", "")
	relJS = strings.ReplaceAll(relJS, "%~dp0\\", "")
	relJS = strings.ReplaceAll(relJS, "%dp0%/", "")
	relJS = strings.ReplaceAll(relJS, "%~dp0/", "")
	targetJS := filepath.Join(filepath.Dir(batchFile), filepath.FromSlash(relJS))
	if !util.Exists(targetJS) {
		t.Fatalf("targetJS does not exist: %q", targetJS)
	}
	nodeExe := util.ResolveNodeBinary()
	if nodeExe == "" {
		if lp, err := exec.LookPath("node"); err == nil {
			nodeExe = lp
		}
	}
	if nodeExe == "" {
		t.Fatal("nodeExe empty")
	}

	exe, args := resolveMcpCommand("cmd", []string{"cmd", "/c", batchFile, "serve", "--mcp"})
	exe2, args2, ok := resolveWindowsBatchToNode("cmd", []string{"cmd", "/c", batchFile, "serve", "--mcp"})
	if !strings.HasSuffix(strings.ToLower(exe), "node.exe") && !strings.HasSuffix(strings.ToLower(exe), "node") {
		t.Fatalf("expected node executable, got: %q (direct call ok=%v exe2=%q args2=%#v)", exe, ok, exe2, args2)
	}
	if len(args) != 3 || args[0] != jsFile || args[1] != "serve" || args[2] != "--mcp" {
		t.Fatalf("unexpected args: %#v", args)
	}

	out, err := exec.Command(exe, args[0]).CombinedOutput()
	if err != nil {
		t.Fatalf("exec node failed: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "shim-executed-ok") {
		t.Fatalf("unexpected node execution output: %s", out)
	}
}
