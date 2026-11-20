package headroom

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPatchCloudCodeGeminiAlreadyPatchedIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gemini.py")
	input := "existing\n" + cloudCodePatchMarker + "\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeGemini(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("already patched file reported changed")
	}
	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != input {
		t.Fatalf("file changed: %q", output)
	}
}

func TestPatchCloudCodeGeminiMissingAnchorLeavesFileUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gemini.py")
	input := "        contents = request_payload.get(\"contents\", [])\n        headers = dict(request.headers.items())\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeGemini(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("missing-anchor file reported changed")
	}
	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != input {
		t.Fatalf("file changed: %q", output)
	}
}

func TestPatchCloudCodeProxyRoutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy_routes.py")
	input := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeProxyRoutes(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected proxy_routes.py to be patched")
	}
	// Idempotency check
	changedAgain, err := patchCloudCodeProxyRoutes(path)
	if err != nil {
		t.Fatal(err)
	}
	if changedAgain {
		t.Fatal("expected second patchCloudCodeProxyRoutes call to be a no-op")
	}
}
