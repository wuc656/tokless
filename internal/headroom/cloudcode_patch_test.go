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
	losslessPath := filepath.Join(site, "headroom", "transforms", "lossless_compaction.py")
	prefixPath := filepath.Join(site, "headroom", "cache", "prefix_tracker.py")
	routerPath := filepath.Join(site, "headroom", "transforms", "content_router.py")
	for _, p := range []string{geminiPath, routeSpecsPath, runtimePath, routesPath, openaiPath, losslessPath, prefixPath, routerPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	runtimeSrc := "CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith((CLOUDCODE_INTERNAL_PREFIX, CLOUDCODE_VERSIONED_INTERNAL_PREFIX)):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n"
	routesSrc := "        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n"
	losslessSrc := "_TIMESTAMP_ROW_RE = re.compile(\n    r\"^\\s*\\[?(?:\"\n    r\"\\d{4}-\\d{2}-\\d{2}[ T]\\d{1,2}:\\d{2}\"  # 2026-09-02 14:30 / ISO 8601 'T'\n    r\"|\\d{2}/\\d{2}/\\d{2,4}[ T]\\d{1,2}:\\d{2}\"  # 09/02/2026 14:30\n    r\"|[A-Z][a-z]{2}\\s+\\d{1,2}\\s+\\d{1,2}:\\d{2}\"  # syslog: Aug 16 11:02\n    r\"|\\d{1,2}:\\d{2}:\\d{2}(?:[.,]\\d+)?(?:\\s|\\]|$)\"  # bare 15:03:53 / [15:03:53]\n    r\")\"\n)\n\ndef search_dir_heading(text: str) -> str:\n    \"\"\"Fold grep ``path:line:content`` rows by DIRECTORY.\n\n    Consecutive rows whose path shares a parent directory collapse to that\n    directory once (a header ending in ``/``), then ``base:line:content`` rows\n    beneath it. Complements :func:`search_heading` (which factors a repeated\n    *file*): this factors a repeated *directory* across distinct files — the\n    common ``grep -rn`` case where each file has a single match, so file-heading\n    saves nothing but the shared directory repeats on every row. Rows whose path\n    has no ``/`` pass through untouched. Exactly reversed by\n    :func:`search_dir_unheading`; ``compact_lossless`` verifies the round-trip.\n    \"\"\"\n    lines, had_trailing = _split_keep_trailing(text)\n    if not lines:\n        return text\n    out: list[str] = []\n    current_dir: str | None = None\n    for line in lines:\n        m = None if _TIMESTAMP_ROW_RE.match(line) else _GREP_ROW_RE.match(line)\n        if m and \"/\" in m.group(\"path\"):\n            path = m.group(\"path\")\n            cut = path.rindex(\"/\") + 1\n            dir_part, base = path[:cut], path[cut:]\n            if dir_part != current_dir:\n                out.append(dir_part)\n                current_dir = dir_part\n            out.append(f\"{base}:{m.group('line')}:{m.group('content')}\")\n        else:\n            out.append(line)\n            current_dir = None\n    return _join(out, had_trailing)\n\n\ndef search_dir_unheading(text: str) -> str:\n    \"\"\"Exact inverse of :func:`search_dir_heading`.\n\n    A *header* is a line ending in ``/`` immediately followed by a\n    ``base:line:content`` data row; it is consumed and re-prefixed onto each\n    following data row until a non-data line appears.\n    \"\"\"\n    lines, had_trailing = _split_keep_trailing(text)\n    if not lines:\n        return text\n    out: list[str] = []\n    current_dir: str | None = None\n    n = len(lines)\n    i = 0\n    while i < n:\n        line = lines[i]\n        data = _DIR_DATA_RE.match(line)\n        if current_dir is not None and data:\n            out.append(f\"{current_dir}{line}\")\n            i += 1\n            continue\n        if line.endswith(\"/\") and i + 1 < n and _DIR_DATA_RE.match(lines[i + 1]):\n            current_dir = line\n            i += 1\n            continue\n        current_dir = None\n        out.append(line)\n        i += 1\n    return _join(out, had_trailing)\n\n        if kind == \"search\":\n            # Two independent folds; keep the smaller that round-trips exactly.\n            # search_heading factors a repeated FILE (many matches in one file);\n            # search_dir_heading factors a repeated DIRECTORY (one match each\n            # across many files in a dir — the grep -rn case the file fold misses).\n            best = content\n            for candidate, inverse in (\n                (search_heading(content), search_unheading),\n                (search_dir_heading(content), search_dir_unheading),\n            ):\n                if inverse(candidate) == content and _smaller(candidate, best):\n                    best = candidate\n            return best\n"
	prefixSrc := "    # Defensive: if a transform dropped prefix messages, restore them.\n    if len(restored) < frozen:\n        return list(original_messages[:frozen]) + restored, frozen\n"
	routerSrc := "                            if compressed == content:\n                                from headroom._core import minify_json\n\n                                minified = minify_json(content)\n                                if minified is not None:\n"

	for _, f := range []struct {
		path string
		body string
	}{
		{geminiPath, testCloudCodeGeminiFixture},
		{routeSpecsPath, testCloudCodeRouteSpecsFixture},
		{runtimePath, runtimeSrc},
		{routesPath, routesSrc},
		{openaiPath, testCloudCodeOpenAIFixture},
		{losslessPath, losslessSrc},
		{prefixPath, prefixSrc},
		{routerPath, routerSrc},
	} {
		if err := os.WriteFile(f.path, []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !applyCloudCodePatch() {
		t.Fatal("expected applyCloudCodePatch to patch files under Roaming uv tools dir")
	}
	if applied, total := CloudCodePatchStatus(); total != 11 || applied != 11 {
		t.Fatalf("CloudCodePatchStatus() = (%d, %d), want (11, 11)", applied, total)
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
		{geminiPath, cloudCodeGeminiFRLosslessMarker},
		{routeSpecsPath, cloudCodeRouteSpecsMarker},
		{runtimePath, cloudCodeRuntimePatchMarker},
		{routesPath, cloudCodeRoutesPatchMarker},
		{openaiPath, cloudCodeStreamingPatchMarker},
		{losslessPath, cloudCodeLosslessWindowsMarker},
		{prefixPath, cloudCodePrefixTrackerMarker},
		{routerPath, cloudCodeSmartCrusherMarker},
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

func TestPatchCloudCodeGeminiFRLossless(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gemini.py")
	if err := os.WriteFile(path, []byte(testCloudCodeGeminiFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := patchCloudCodeGemini(path); err != nil || !changed {
		t.Fatalf("patchCloudCodeGemini failed: changed=%v err=%v", changed, err)
	}
	if changed, err := patchCloudCodeGeminiFRLossless(path); err != nil || !changed {
		t.Fatalf("patchCloudCodeGeminiFRLossless failed: changed=%v err=%v", changed, err)
	}
	if changedAgain, err := patchCloudCodeGeminiFRLossless(path); err != nil || changedAgain {
		t.Fatalf("expected idempotent patchCloudCodeGeminiFRLossless, changed=%v err=%v", changedAgain, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{cloudCodeGeminiFRLosslessMarker, "_compact_fr_str", "compact_lossless", `"functionResponse"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("patched gemini.py missing %q", want)
		}
	}
}

func TestPatchCloudCodeUpstreamTransformsAndPrefixTracker(t *testing.T) {
	dir := t.TempDir()
	prefixPath := filepath.Join(dir, "prefix_tracker.py")
	prefixSrc := "    # Defensive: if a transform dropped prefix messages, restore them.\n    if len(restored) < frozen:\n        return list(original_messages[:frozen]) + restored, frozen\n"
	if err := os.WriteFile(prefixPath, []byte(prefixSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := patchCloudCodePrefixTracker(prefixPath); err != nil || !changed {
		t.Fatalf("patchCloudCodePrefixTracker failed: changed=%v err=%v", changed, err)
	}
	if changedAgain, err := patchCloudCodePrefixTracker(prefixPath); err != nil || changedAgain {
		t.Fatalf("expected idempotent patchCloudCodePrefixTracker, changed=%v err=%v", changedAgain, err)
	}

	routerPath := filepath.Join(dir, "content_router.py")
	routerSrc := "                            if compressed == content:\n                                from headroom._core import minify_json\n\n                                minified = minify_json(content)\n                                if minified is not None:\n"
	if err := os.WriteFile(routerPath, []byte(routerSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := patchCloudCodeSmartCrusherMinify(routerPath); err != nil || !changed {
		t.Fatalf("patchCloudCodeSmartCrusherMinify failed: changed=%v err=%v", changed, err)
	}
	if changedAgain, err := patchCloudCodeSmartCrusherMinify(routerPath); err != nil || changedAgain {
		t.Fatalf("expected idempotent patchCloudCodeSmartCrusherMinify, changed=%v err=%v", changedAgain, err)
	}
}

func TestPatchCloudCodeRealSitePackagesInTempDir(t *testing.T) {
	realSite := filepath.Join(os.Getenv("LOCALAPPDATA"), "tokless", "headroom", "tools", "headroom-ai", "Lib", "site-packages")
	if !util.Exists(realSite) {
		t.Skip("real headroom site-packages not present")
	}
	tmp := t.TempDir()
	files := []struct {
		rel    string
		fn     func(string) (bool, error)
		marker string
	}{
		{filepath.Join("headroom", "proxy", "handlers", "gemini.py"), patchCloudCodeGeminiFRLossless, cloudCodeGeminiFRLosslessMarker},
		{filepath.Join("headroom", "transforms", "lossless_compaction.py"), patchCloudCodeLosslessWindows, cloudCodeLosslessWindowsMarker},
		{filepath.Join("headroom", "cache", "prefix_tracker.py"), patchCloudCodePrefixTracker, cloudCodePrefixTrackerMarker},
		{filepath.Join("headroom", "transforms", "content_router.py"), patchCloudCodeSmartCrusherMinify, cloudCodeSmartCrusherMarker},
	}
	for _, f := range files {
		src := filepath.Join(realSite, f.rel)
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Skipf("skipping %s: %v", f.rel, err)
		}
		dst := filepath.Join(tmp, filepath.Base(f.rel))
		// Normalize CRLF -> LF if needed before testing, or test raw directly
		if err := os.WriteFile(dst, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), f.marker) {
			changed, err := f.fn(dst)
			if err != nil || !changed {
				t.Fatalf("patch %s failed on real file copy: changed=%v err=%v", f.rel, changed, err)
			}
		}
		patched, err := os.ReadFile(dst)
		if err != nil || !strings.Contains(string(patched), f.marker) {
			t.Fatalf("patched %s missing marker %q", f.rel, f.marker)
		}
		pyExe := filepath.Join(os.Getenv("LOCALAPPDATA"), "tokless", "headroom", "tools", "headroom-ai", "Scripts", "python.exe")
		if util.Exists(pyExe) {
			res := util.Run(pyExe, []string{"-m", "py_compile", dst}, util.RunOptions{Capture: true})
			if res.Code != 0 {
				t.Fatalf("py_compile failed for patched %s: %s", f.rel, res.Stderr)
			}
		}
	}
}
