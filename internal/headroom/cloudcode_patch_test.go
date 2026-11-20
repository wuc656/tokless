package headroom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HoangP8/tokless/internal/util"
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
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), cloudCodeRoutesPatchMarker) || !strings.Contains(string(out), "_handle_streaming_passthrough") {
		t.Fatalf("unexpected patched proxy_routes.py:\n%s", out)
	}
}

func TestPatchCloudCodeProxyRoutesUpgradesV1ToV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy_routes.py")
	v1 := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            if \"antigravity\" in request.headers.get(\"user-agent\", \"\").lower():\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n            )\n"
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeProxyRoutes(path)
	if err != nil || !changed {
		t.Fatalf("expected v1 -> v2 upgrade, changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), cloudCodeRoutesPatchMarker) || !strings.Contains(string(out), "jetski") {
		t.Fatalf("expected v2 content, got:\n%s", out)
	}
	if changedAgain, err := patchCloudCodeProxyRoutes(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent v2, changed=%v err=%v", changedAgain, err)
	}
}

func TestPatchCloudCodeRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.py")
	input := "CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith((CLOUDCODE_INTERNAL_PREFIX, CLOUDCODE_VERSIONED_INTERNAL_PREFIX)):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeRuntime(path)
	if err != nil || !changed {
		t.Fatalf("patchCloudCodeRuntime: changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{cloudCodeRuntimePatchMarker, "v1/v1internal/", "google.internal.cloud.code."} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched runtime.py missing %q:\n%s", want, out)
		}
	}
	if changedAgain, err := patchCloudCodeRuntime(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent runtime.py, changed=%v err=%v", changedAgain, err)
	}
}

func TestPatchCloudCodeGeminiRouting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gemini.py")
	input := "    def _is_cloudcode_antigravity_request(\n        self, body: dict[str, Any], headers: dict[str, str]\n    ) -> bool:\n        \"\"\"Detect Pi/OpenClaw antigravity requests routed via Cloud Code Assist.\"\"\"\n        user_agent = headers.get(\"user-agent\", \"\").lower()\n        body_user_agent = str(body.get(\"userAgent\", \"\")).lower()\n        return (\n            body.get(\"requestType\") == \"agent\"\n            or body_user_agent == \"antigravity\"\n            or user_agent.startswith(\"antigravity/\")\n        )\n\n    def _resolve_cloudcode_base_url(self, is_antigravity: bool) -> str:\n        \"\"\"Resolve upstream base URL for Pi Cloud Code Assist / Antigravity traffic.\"\"\"\n        if is_antigravity:\n            return ANTIGRAVITY_DAILY_API_URL\n        return getattr(self, \"CLOUDCODE_API_URL\", DEFAULT_CLOUDCODE_API_URL).rstrip(\"/\")\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeGeminiRouting(path)
	if err != nil || !changed {
		t.Fatalf("patchCloudCodeGeminiRouting: changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{cloudCodeGeminiRoutingMarker, "x-goog-api-client", "jetski", "configured != DEFAULT_CLOUDCODE_API_URL"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched gemini.py missing %q:\n%s", want, out)
		}
	}
	if changedAgain, err := patchCloudCodeGeminiRouting(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent gemini.py routing, changed=%v err=%v", changedAgain, err)
	}
}

func TestApplyCloudCodePatchDiscoversWindowsRoamingUVToolDir(t *testing.T) {
	origIsWin := util.IsWin
	util.IsWin = true
	t.Cleanup(func() { util.IsWin = origIsWin })

	home := t.TempDir()
	util.SetHomeOverride(home)
	t.Cleanup(func() { util.SetHomeOverride("") })
	t.Setenv("UV_TOOL_DIR", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("APPDATA", `C:\should\not\be\used\in\test`)

	site := filepath.Join(home, "AppData", "Roaming", "uv", "tools", "headroom-ai", "Lib", "site-packages")
	geminiPath := filepath.Join(site, "headroom", "proxy", "handlers", "gemini.py")
	runtimePath := filepath.Join(site, "headroom", "providers", "cloudcode", "runtime.py")
	routesPath := filepath.Join(site, "headroom", "providers", "proxy_routes.py")
	for _, p := range []string{geminiPath, runtimePath, routesPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	geminiSrc := "    def _is_cloudcode_antigravity_request(\n        self, body: dict[str, Any], headers: dict[str, str]\n    ) -> bool:\n        \"\"\"Detect Pi/OpenClaw antigravity requests routed via Cloud Code Assist.\"\"\"\n        user_agent = headers.get(\"user-agent\", \"\").lower()\n        body_user_agent = str(body.get(\"userAgent\", \"\")).lower()\n        return (\n            body.get(\"requestType\") == \"agent\"\n            or body_user_agent == \"antigravity\"\n            or user_agent.startswith(\"antigravity/\")\n        )\n\n    def _resolve_cloudcode_base_url(self, is_antigravity: bool) -> str:\n        \"\"\"Resolve upstream base URL for Pi Cloud Code Assist / Antigravity traffic.\"\"\"\n        if is_antigravity:\n            return ANTIGRAVITY_DAILY_API_URL\n        return getattr(self, \"CLOUDCODE_API_URL\", DEFAULT_CLOUDCODE_API_URL).rstrip(\"/\")\n"
	runtimeSrc := "CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith((CLOUDCODE_INTERNAL_PREFIX, CLOUDCODE_VERSIONED_INTERNAL_PREFIX)):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n"
	routesSrc := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n"

	if err := os.WriteFile(geminiPath, []byte(geminiSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte(runtimeSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesPath, []byte(routesSrc), 0o600); err != nil {
		t.Fatal(err)
	}

	if !applyCloudCodePatch() {
		t.Fatal("expected applyCloudCodePatch to patch files under Roaming uv tools dir")
	}
	if applyCloudCodePatch() {
		t.Fatal("expected second applyCloudCodePatch call to be idempotent")
	}
	for _, tc := range []struct {
		path   string
		marker string
	}{
		{geminiPath, cloudCodeGeminiRoutingMarker},
		{runtimePath, cloudCodeRuntimePatchMarker},
		{routesPath, cloudCodeRoutesPatchMarker},
	} {
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), tc.marker) {
			t.Fatalf("%s missing marker %q after applyCloudCodePatch", tc.path, tc.marker)
		}
	}
}
