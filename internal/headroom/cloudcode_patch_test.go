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
	for _, want := range []string{cloudCodeRoutesPatchMarker, "_handle_streaming_passthrough", "application/connect+", "google.cloud.cloudaicompanion."} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched proxy_routes.py missing %q:\n%s", want, out)
		}
	}
}

func TestPatchCloudCodeProxyRoutesUpgradesV1ToV3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy_routes.py")
	v1 := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            if \"antigravity\" in request.headers.get(\"user-agent\", \"\").lower():\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n            )\n"
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeProxyRoutes(path)
	if err != nil || !changed {
		t.Fatalf("expected v1 -> v3 upgrade, changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), cloudCodeRoutesPatchMarker) || !strings.Contains(string(out), "jetski") || !strings.Contains(string(out), "application/grpc") {
		t.Fatalf("expected v3 content, got:\n%s", out)
	}
	if changedAgain, err := patchCloudCodeProxyRoutes(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent v3, changed=%v err=%v", changedAgain, err)
	}
}

func TestPatchCloudCodeProxyRoutesUpgradesV2ToV3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy_routes.py")
	v2 := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            # tokless:cloudcode_routes_v2\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            _cc_ua = request.headers.get(\"user-agent\", \"\").lower()\n            _cc_api = request.headers.get(\"x-goog-api-client\", \"\").lower()\n            _cc_client = request.headers.get(\"x-client\", \"\").lower()\n            _cc_antigravity = (\n                \"antigravity\" in _cc_ua\n                or \"jetski\" in _cc_ua\n                or \"antigravity\" in _cc_api\n                or \"jetski\" in _cc_api\n                or _cc_client == \"antigravity\"\n                or normalized_cloudcode_path.startswith(\"/google.internal.cloud.code.\")\n            )\n            if _cc_antigravity and _cloudcode_target.rstrip(\"/\") == \"https://cloudcode-pa.googleapis.com\":\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n            _cc_stream = (\n                \":stream\" in normalized_cloudcode_path.lower()\n                or request.query_params.get(\"alt\", \"\").lower() == \"sse\"\n                or \"text/event-stream\" in request.headers.get(\"accept\", \"\").lower()\n            )\n            if _cc_stream:\n                return await proxy._handle_streaming_passthrough(\n                    request=request,\n                    base_url=_cloudcode_target,\n                    endpoint_name=\"cloudcode_stream\",\n                    provider=\"gemini\",\n                )\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n                \"cloudcode_passthrough\",\n                \"gemini\",\n            )\n"
	if err := os.WriteFile(path, []byte(v2), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeProxyRoutes(path)
	if err != nil || !changed {
		t.Fatalf("expected v2 -> v3 upgrade, changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{cloudCodeRoutesPatchMarker, "connect-protocol-version", "application/x-ndjson", "application/connect+"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("upgraded v3 proxy_routes.py missing %q:\n%s", want, out)
		}
	}
	if changedAgain, err := patchCloudCodeProxyRoutes(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent v3 after upgrade, changed=%v err=%v", changedAgain, err)
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

const testCloudCodeGeminiFixture = `    def _is_cloudcode_antigravity_request(
        self, body: dict[str, Any], headers: dict[str, str]
    ) -> bool:
        """Detect Pi/OpenClaw antigravity requests routed via Cloud Code Assist."""
        user_agent = headers.get("user-agent", "").lower()
        body_user_agent = str(body.get("userAgent", "")).lower()
        return (
            body.get("requestType") == "agent"
            or body_user_agent == "antigravity"
            or user_agent.startswith("antigravity/")
        )

    def _resolve_cloudcode_base_url(self, is_antigravity: bool) -> str:
        """Resolve upstream base URL for Pi Cloud Code Assist / Antigravity traffic."""
        if is_antigravity:
            return ANTIGRAVITY_DAILY_API_URL
        return getattr(self, "CLOUDCODE_API_URL", DEFAULT_CLOUDCODE_API_URL).rstrip("/")

    async def handle_google_cloudcode_stream(
        self,
        request: Request,
    ) -> StreamingResponse | JSONResponse:
        """Handle Pi/OpenClaw Google Cloud Code Assist and Antigravity streaming requests."""
        from fastapi import HTTPException
        from fastapi.responses import JSONResponse

        from headroom.proxy.helpers import _read_request_json
        from headroom.utils import extract_user_query

        start_time = time.time()
        request_id = await self._next_request_id()

        try:
            body = await _read_request_json(request)
        except ValueError:
            pass
        contents = request_payload.get("contents", [])
        headers = dict(request.headers.items())
        if not messages:
            original_tokens = 0
        optimized_messages = messages
            if not is_antigravity:
                if optimized_system:
                    request_payload["systemInstruction"] = optimized_system
                elif "systemInstruction" in request_payload:
                    del request_payload["systemInstruction"]

        tokens_saved = original_tokens - optimized_tokens
        optimization_latency = (time.time() - start_time) * 1000
        base_url = self._resolve_cloudcode_base_url(is_antigravity)
        stream_url = f"{base_url}/v1internal:streamGenerateContent"
        if request.url.query:
            stream_url = f"{stream_url}?{request.url.query}"

        return await self._stream_response(
            stream_url,
            headers,
            body,
            "gemini",
            model,
            request_id,
            original_tokens,
            optimized_tokens,
            tokens_saved,
            transforms_applied,
            tags,
            optimization_latency,
            prefix_tracker=gemini_prefix_tracker,
            original_messages=messages,
        )
`

const testCloudCodeRouteSpecsFixture = `CLOUDCODE_HANDLER_ROUTES: tuple[ProviderHandlerRoute, ...] = (
    ProviderHandlerRoute(
        "POST",
        "/v1internal:streamGenerateContent",
        "handle_google_cloudcode_stream",
    ),
    ProviderHandlerRoute(
        "POST",
        "/v1/v1internal:streamGenerateContent",
        "handle_google_cloudcode_stream",
    ),
)
`

const testCloudCodeOpenAIFixture = `    async def handle_passthrough(self, request):
        headers = dict(request.headers.items())
        headers.pop("host", None)
        headers.pop("accept-encoding", None)
        client = classify_client(headers)
        tags = extract_tags(headers)
        # PR-A5 (P5-49): strip internal x-headroom-* before forwarding upstream.

    async def _handle_streaming_passthrough(self, request):
        headers = dict(request.headers.items())
        headers.pop("host", None)
        headers.pop("accept-encoding", None)
        client = classify_client(headers)
        tags = extract_tags(headers)
        if _upstream_response is not None:
            pass

        async def generate():
            try:
                async with contextlib.aclosing(upstream_response) as response:
                    async for chunk in response.aiter_bytes():
                        if stream_state["ttfb_ms"] is None:
                            stream_state["ttfb_ms"] = (time.time() - start_time) * 1000
                        stream_state["total_bytes"] += len(chunk)
                        stream_state["sse_buffer"].extend(chunk)
                        if len(stream_state["sse_buffer"]) > MAX_SSE_BUFFER_SIZE:
                            tail = bytes(stream_state["sse_buffer"][-MAX_SSE_BUFFER_SIZE // 2 :])
                            stream_state["sse_buffer"] = bytearray(tail)

                        _absorb_usage(
                            self._parse_sse_usage_from_buffer(stream_state, stream_provider)
                        )
                        yield chunk
            finally:
                completed_normally = True
                buf = stream_state["sse_buffer"]
                if len(buf) > 0:
                    buf.extend(b"\n\n")
                    # A truncated stream can end inside a multi-byte character;
                    # the strict decoder then raises and would skip the outcome
                    # below. Keep the usage already parsed, drop the unusable
                    # tail, and report the stream as failed.
                    try:
                        _absorb_usage(
                            self._parse_sse_usage_from_buffer(stream_state, stream_provider)
                        )
                    except UnicodeDecodeError:
                        completed_normally = False
`

func TestPatchCloudCodeGeminiNonStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gemini.py")
	if err := os.WriteFile(path, []byte(testCloudCodeGeminiFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func(string) (bool, error){
		patchCloudCodeGemini,
		patchCloudCodeGeminiRouting,
		patchCloudCodeGeminiNonStream,
	} {
		changed, err := fn(path)
		if err != nil || !changed {
			t.Fatalf("expected patch to apply, changed=%v err=%v", changed, err)
		}
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		cloudCodePatchMarker,
		cloudCodeGeminiRoutingMarker,
		cloudCodeGeminiNonStreamMarker,
		"async def handle_google_cloudcode_generate(",
		"application/connect+",
		`"proxy-connection"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched gemini.py missing %q", want)
		}
	}
	if changedAgain, err := patchCloudCodeGeminiNonStream(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent gemini.py non-stream patch, changed=%v err=%v", changedAgain, err)
	}

	// Missing second anchor must leave file completely untouched.
	partialPath := filepath.Join(t.TempDir(), "partial_gemini.py")
	partial := "    async def handle_google_cloudcode_stream(\n        self,\n        request: Request,\n    ) -> StreamingResponse | JSONResponse:\n"
	if err := os.WriteFile(partialPath, []byte(partial), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeGeminiNonStream(partialPath)
	if err != nil || changed {
		t.Fatalf("missing anchor must be no-op, changed=%v err=%v", changed, err)
	}
	gotPartial, err := os.ReadFile(partialPath)
	if err != nil || string(gotPartial) != partial {
		t.Fatalf("partial file mutated: %q", gotPartial)
	}
}

func TestPatchCloudCodeRouteSpecs(t *testing.T) {
	root := t.TempDir()
	geminiPath := filepath.Join(root, "headroom", "proxy", "handlers", "gemini.py")
	routeSpecsPath := filepath.Join(root, "headroom", "providers", "route_specs.py")
	for _, p := range []string{geminiPath, routeSpecsPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(geminiPath, []byte(testCloudCodeGeminiFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routeSpecsPath, []byte(testCloudCodeRouteSpecsFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	// Must refuse to register handle_google_cloudcode_generate while gemini.py lacks it.
	changed, err := patchCloudCodeRouteSpecs(routeSpecsPath)
	if err != nil || changed {
		t.Fatalf("expected skip without gemini handler, changed=%v err=%v", changed, err)
	}

	if changedGemini, err := patchCloudCodeGeminiNonStream(geminiPath); err != nil || !changedGemini {
		t.Fatalf("expected gemini non-stream patch, changed=%v err=%v", changedGemini, err)
	}
	changed, err = patchCloudCodeRouteSpecs(routeSpecsPath)
	if err != nil || !changed {
		t.Fatalf("expected route_specs.py patch after gemini.py, changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(routeSpecsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		cloudCodeRouteSpecsMarker,
		`"/v1internal:generateContent"`,
		`"/v1/v1internal:generateContent"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched route_specs.py missing %q:\n%s", want, out)
		}
	}
	if changedAgain, err := patchCloudCodeRouteSpecs(routeSpecsPath); err != nil || changedAgain {
		t.Fatalf("expected idempotent route_specs.py, changed=%v err=%v", changedAgain, err)
	}
}

func TestPatchCloudCodeStreamingPassthrough(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openai.py")
	if err := os.WriteFile(path, []byte(testCloudCodeOpenAIFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := patchCloudCodeStreamingPassthrough(path)
	if err != nil || !changed {
		t.Fatalf("expected openai.py patch, changed=%v err=%v", changed, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		cloudCodeStreamingPatchMarker,
		`_is_sse_resp = _resp_ct.startswith("text/event-stream")`,
		`if _is_sse_resp:`,
		`if _is_sse_resp and len(buf) > 0:`,
		`"proxy-connection"`,
		`if headers.get("te", "").lower() != "trailers":`,
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched openai.py missing %q", want)
		}
	}
	if changedAgain, err := patchCloudCodeStreamingPassthrough(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent openai.py, changed=%v err=%v", changedAgain, err)
	}

	// Missing one anchor must leave file completely unchanged.
	partialPath := filepath.Join(t.TempDir(), "partial_openai.py")
	partial := "        headers = dict(request.headers.items())\n        headers.pop(\"host\", None)\n        headers.pop(\"accept-encoding\", None)\n        client = classify_client(headers)\n        tags = extract_tags(headers)\n        # PR-A5 (P5-49): strip internal x-headroom-* before forwarding upstream.\n"
	if err := os.WriteFile(partialPath, []byte(partial), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err = patchCloudCodeStreamingPassthrough(partialPath)
	if err != nil || changed {
		t.Fatalf("partial openai.py must be no-op, changed=%v err=%v", changed, err)
	}
	gotPartial, err := os.ReadFile(partialPath)
	if err != nil || string(gotPartial) != partial {
		t.Fatalf("partial openai.py mutated: %q", gotPartial)
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
	routeSpecsPath := filepath.Join(site, "headroom", "providers", "route_specs.py")
	runtimePath := filepath.Join(site, "headroom", "providers", "cloudcode", "runtime.py")
	routesPath := filepath.Join(site, "headroom", "providers", "proxy_routes.py")
	openaiPath := filepath.Join(site, "headroom", "proxy", "handlers", "openai.py")
	for _, p := range []string{geminiPath, routeSpecsPath, runtimePath, routesPath, openaiPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	runtimeSrc := "CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith((CLOUDCODE_INTERNAL_PREFIX, CLOUDCODE_VERSIONED_INTERNAL_PREFIX)):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n"
	routesSrc := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n"

	for _, f := range []struct {
		path string
		body string
	}{
		{geminiPath, testCloudCodeGeminiFixture},
		{routeSpecsPath, testCloudCodeRouteSpecsFixture},
		{runtimePath, runtimeSrc},
		{routesPath, routesSrc},
		{openaiPath, testCloudCodeOpenAIFixture},
	} {
		if err := os.WriteFile(f.path, []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !applyCloudCodePatch() {
		t.Fatal("expected applyCloudCodePatch to patch files under Roaming uv tools dir")
	}
	if applied, total := CloudCodePatchStatus(); total != 7 || applied != 7 {
		t.Fatalf("CloudCodePatchStatus() = (%d, %d), want (7, 7)", applied, total)
	}
	if applyCloudCodePatch() {
		t.Fatal("expected second applyCloudCodePatch call to be idempotent")
	}
	for _, tc := range []struct {
		path   string
		marker string
	}{
		{geminiPath, cloudCodePatchMarker},
		{geminiPath, cloudCodeGeminiRoutingMarker},
		{geminiPath, cloudCodeGeminiNonStreamMarker},
		{routeSpecsPath, cloudCodeRouteSpecsMarker},
		{runtimePath, cloudCodeRuntimePatchMarker},
		{routesPath, cloudCodeRoutesPatchMarker},
		{openaiPath, cloudCodeStreamingPatchMarker},
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

func TestApplyCloudCodePatchSkipsWithoutHomeOverrideInTest(t *testing.T) {
	util.SetHomeOverride("")
	t.Cleanup(func() { util.SetHomeOverride("") })

	uvTools := t.TempDir()
	t.Setenv("UV_TOOL_DIR", uvTools)
	routesPath := filepath.Join(uvTools, "headroom-ai", "Lib", "site-packages", "headroom", "providers", "proxy_routes.py")
	if err := os.MkdirAll(filepath.Dir(routesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	routesSrc := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n"
	if err := os.WriteFile(routesPath, []byte(routesSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if applyCloudCodePatch() {
		t.Fatal("applyCloudCodePatch mutated files in test without SetHomeOverride")
	}
	raw, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != routesSrc {
		t.Fatalf("proxy_routes.py was modified without home override")
	}
}
