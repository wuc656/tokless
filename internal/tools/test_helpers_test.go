package tools

import (
	"os"
	"testing"
)

// chdirTemp moves cwd out of the package dir so tests that write
// relative files (.cursor/, AGENTS.md, etc.) don't pollute the git workspace.
func chdirTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}
