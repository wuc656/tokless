package headroom

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/util"
)

const (
	cloudCodePatchMarker         = "gemini:tool_schema_compaction"
	cloudCodeGeminiRoutingMarker = "tokless:antigravity_routing_v1"
	cloudCodeRuntimePatchMarker  = "CLOUDCODE_INTERNAL_SLASH_PREFIX"
	cloudCodeRoutesPatchMarker   = "tokless:cloudcode_routes_v2"
)

func cloudCodePatchRootsFor(rel ...string) []string {
	var patterns []string
	for _, tools := range util.HeadroomUVToolDirs() {
		posixArgs := append([]string{tools, "headroom-ai", "lib", "python*", "site-packages"}, rel...)
		winArgs := append([]string{tools, "headroom-ai", "Lib", "site-packages"}, rel...)
		patterns = append(patterns, filepath.Join(posixArgs...), filepath.Join(winArgs...))
	}
	return patterns
}

func cloudCodePatchRoots() []string {
	return cloudCodePatchRootsFor("headroom", "proxy", "handlers", "gemini.py")
}

func cloudCodeRoutesPatchRoots() []string {
	return cloudCodePatchRootsFor("headroom", "providers", "proxy_routes.py")
}

func writePatchedFile(path string, patched []byte) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := util.WriteFileAtomic(path, string(patched), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

func patchCloudCodeGemini(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(cloudCodePatchMarker)) {
		return false, nil
	}

	type replacement struct{ old, new []byte }
	replacements := []replacement{
		{[]byte("        contents = request_payload.get(\"contents\", [])\n        headers = dict(request.headers.items())\n"), []byte("        contents = request_payload.get(\"contents\", [])\n        tools_before = request_payload.get(\"tools\")\n        tools_after = tools_before\n        tools_modified = False\n        headers = dict(request.headers.items())\n")},
		{[]byte("        if not messages:\n            original_tokens = 0\n        optimized_messages = messages\n"), []byte("        if not messages:\n            original_tokens = 0\n        _tool_tokens_before = 0\n        _tool_tokens_after = 0\n        optimized_messages = messages\n")},
		{[]byte("            if not is_antigravity:\n                if optimized_system:\n                    request_payload[\"systemInstruction\"] = optimized_system\n                elif \"systemInstruction\" in request_payload:\n                    del request_payload[\"systemInstruction\"]\n\n        tokens_saved = original_tokens - optimized_tokens\n        optimization_latency = (time.time() - start_time) * 1000\n        base_url = self._resolve_cloudcode_base_url(is_antigravity)\n"), []byte("            if optimized_system:\n                request_payload[\"systemInstruction\"] = optimized_system\n\n        if not _decision.bypass_header_set and isinstance(tools_before, list) and tools_before:\n            try:\n                from headroom.proxy.tool_schema_compaction import compact_tools\n\n                compacted_payload, tools_modified, _, _ = compact_tools(\n                    {\"tools\": tools_before}\n                )\n                tools_after = compacted_payload.get(\"tools\", tools_before)\n                if tools_modified:\n                    request_payload[\"tools\"] = tools_after\n            except Exception:\n                tools_modified = False\n                tools_after = tools_before\n                logger.debug(\"Cloud Code tool schema compaction skipped\", exc_info=True)\n            if tools_modified:\n                try:\n                    _tool_tokens_before = tokenizer.count_text(\n                        json.dumps(tools_before, default=str)\n                    )\n                    _tool_tokens_after = tokenizer.count_text(\n                        json.dumps(tools_after, default=str)\n                    )\n                except Exception:\n                    logger.debug(\"Cloud Code tool token recount skipped\", exc_info=True)\n\n        if 0 < _tool_tokens_after < _tool_tokens_before:\n            original_tokens += _tool_tokens_before\n            optimized_tokens += _tool_tokens_after\n            transforms_applied.append(\"gemini:tool_schema_compaction\")\n        tokens_saved = max(0, original_tokens - optimized_tokens)\n        optimization_latency = (time.time() - start_time) * 1000\n        base_url = self._resolve_cloudcode_base_url(is_antigravity)\n")},
	}

	patched := data
	for _, r := range replacements {
		if bytes.Count(patched, r.old) != 1 {
			return false, nil
		}
		patched = bytes.Replace(patched, r.old, r.new, 1)
	}
	return writePatchedFile(path, patched)
}

func patchCloudCodeGeminiRouting(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(cloudCodeGeminiRoutingMarker)) {
		return false, nil
	}

	old := []byte("    def _is_cloudcode_antigravity_request(\n        self, body: dict[str, Any], headers: dict[str, str]\n    ) -> bool:\n        \"\"\"Detect Pi/OpenClaw antigravity requests routed via Cloud Code Assist.\"\"\"\n        user_agent = headers.get(\"user-agent\", \"\").lower()\n        body_user_agent = str(body.get(\"userAgent\", \"\")).lower()\n        return (\n            body.get(\"requestType\") == \"agent\"\n            or body_user_agent == \"antigravity\"\n            or user_agent.startswith(\"antigravity/\")\n        )\n\n    def _resolve_cloudcode_base_url(self, is_antigravity: bool) -> str:\n        \"\"\"Resolve upstream base URL for Pi Cloud Code Assist / Antigravity traffic.\"\"\"\n        if is_antigravity:\n            return ANTIGRAVITY_DAILY_API_URL\n        return getattr(self, \"CLOUDCODE_API_URL\", DEFAULT_CLOUDCODE_API_URL).rstrip(\"/\")\n")
	new := []byte("    def _is_cloudcode_antigravity_request(\n        self, body: dict[str, Any], headers: dict[str, str]\n    ) -> bool:\n        \"\"\"Detect Antigravity/Jetski requests routed via Cloud Code Assist (" + cloudCodeGeminiRoutingMarker + ").\"\"\"\n        user_agent = headers.get(\"user-agent\", \"\").lower()\n        api_client = headers.get(\"x-goog-api-client\", \"\").lower()\n        x_client = headers.get(\"x-client\", \"\").lower()\n        body_user_agent = str(body.get(\"userAgent\", \"\")).lower()\n        return (\n            body.get(\"requestType\") == \"agent\"\n            or \"antigravity\" in body_user_agent\n            or \"jetski\" in body_user_agent\n            or \"antigravity\" in user_agent\n            or \"jetski\" in user_agent\n            or \"antigravity\" in api_client\n            or \"jetski\" in api_client\n            or x_client == \"antigravity\"\n        )\n\n    def _resolve_cloudcode_base_url(self, is_antigravity: bool) -> str:\n        \"\"\"Resolve upstream base URL for Cloud Code Assist / Antigravity traffic.\"\"\"\n        configured = getattr(self, \"CLOUDCODE_API_URL\", DEFAULT_CLOUDCODE_API_URL).rstrip(\"/\")\n        if configured != DEFAULT_CLOUDCODE_API_URL.rstrip(\"/\"):\n            return configured\n        if is_antigravity:\n            return ANTIGRAVITY_DAILY_API_URL\n        return configured\n")

	if bytes.Count(data, old) != 1 {
		return false, nil
	}
	return writePatchedFile(path, bytes.Replace(data, old, new, 1))
}

func patchCloudCodeRuntime(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(cloudCodeRuntimePatchMarker)) {
		return false, nil
	}

	old := []byte("CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith((CLOUDCODE_INTERNAL_PREFIX, CLOUDCODE_VERSIONED_INTERNAL_PREFIX)):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n")
	new := []byte("CLOUDCODE_INTERNAL_PREFIX = \"v1internal:\"\nCLOUDCODE_VERSIONED_INTERNAL_PREFIX = \"v1/v1internal:\"\nCLOUDCODE_INTERNAL_SLASH_PREFIX = \"v1internal/\"\nCLOUDCODE_VERSIONED_INTERNAL_SLASH_PREFIX = \"v1/v1internal/\"\nCLOUDCODE_RPC_PREFIX = \"google.internal.cloud.code.\"\n\n\ndef normalize_cloudcode_passthrough_path(path: str) -> str | None:\n    \"\"\"Return the canonical Cloud Code internal path, or ``None`` when unrelated.\"\"\"\n    clean_path = path.lstrip(\"/\")\n    if not clean_path.startswith(\n        (\n            CLOUDCODE_INTERNAL_PREFIX,\n            CLOUDCODE_VERSIONED_INTERNAL_PREFIX,\n            CLOUDCODE_INTERNAL_SLASH_PREFIX,\n            CLOUDCODE_VERSIONED_INTERNAL_SLASH_PREFIX,\n            CLOUDCODE_RPC_PREFIX,\n        )\n    ):\n        return None\n    if clean_path.startswith(\"v1/\"):\n        clean_path = clean_path[3:]\n    return f\"/{clean_path}\"\n")

	if bytes.Count(data, old) != 1 {
		return false, nil
	}
	return writePatchedFile(path, bytes.Replace(data, old, new, 1))
}

func patchCloudCodeProxyRoutes(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(cloudCodeRoutesPatchMarker)) {
		return false, nil
	}

	oldOriginal := []byte("        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n\n            return await proxy.handle_passthrough(\n                request,\n                _api_target(proxy, \"cloudcode\"),\n            )\n")
	oldV1 := []byte("        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            if \"antigravity\" in request.headers.get(\"user-agent\", \"\").lower():\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n            )\n")
	new := []byte("        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            # " + cloudCodeRoutesPatchMarker + "\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            _cc_ua = request.headers.get(\"user-agent\", \"\").lower()\n            _cc_api = request.headers.get(\"x-goog-api-client\", \"\").lower()\n            _cc_client = request.headers.get(\"x-client\", \"\").lower()\n            _cc_antigravity = (\n                \"antigravity\" in _cc_ua\n                or \"jetski\" in _cc_ua\n                or \"antigravity\" in _cc_api\n                or \"jetski\" in _cc_api\n                or _cc_client == \"antigravity\"\n                or normalized_cloudcode_path.startswith(\"/google.internal.cloud.code.\")\n            )\n            if _cc_antigravity and _cloudcode_target.rstrip(\"/\") == \"https://cloudcode-pa.googleapis.com\":\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n            _cc_stream = (\n                \":stream\" in normalized_cloudcode_path.lower()\n                or request.query_params.get(\"alt\", \"\").lower() == \"sse\"\n                or \"text/event-stream\" in request.headers.get(\"accept\", \"\").lower()\n            )\n            if _cc_stream:\n                return await proxy._handle_streaming_passthrough(\n                    request=request,\n                    base_url=_cloudcode_target,\n                    endpoint_name=\"cloudcode_stream\",\n                    provider=\"gemini\",\n                )\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n                \"cloudcode_passthrough\",\n                \"gemini\",\n            )\n")

	var target []byte
	switch {
	case bytes.Count(data, oldOriginal) == 1:
		target = oldOriginal
	case bytes.Count(data, oldV1) == 1:
		target = oldV1
	default:
		return false, nil
	}
	return writePatchedFile(path, bytes.Replace(data, target, new, 1))
}

func applyPatchToRoots(patterns []string, label string, patchFns ...func(string) (bool, error)) bool {
	changed := false
	seen := map[string]bool{}
	for _, pattern := range patterns {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range paths {
			clean := filepath.Clean(path)
			key := clean
			if util.IsWin {
				key = strings.ToLower(clean)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			for _, fn := range patchFns {
				patched, err := fn(clean)
				if err != nil {
					util.L.Sub("headroom " + label + " patch skipped: " + err.Error())
					continue
				}
				changed = changed || patched
			}
		}
	}
	return changed
}

func applyCloudCodePatch() bool {
	changed := applyPatchToRoots(
		cloudCodePatchRoots(),
		"Cloud Code",
		patchCloudCodeGemini,
		patchCloudCodeGeminiRouting,
	)
	changed = applyPatchToRoots(
		cloudCodePatchRootsFor("headroom", "providers", "cloudcode", "runtime.py"),
		"Cloud Code runtime",
		patchCloudCodeRuntime,
	) || changed
	changed = applyPatchToRoots(
		cloudCodeRoutesPatchRoots(),
		"Cloud Code routes",
		patchCloudCodeProxyRoutes,
	) || changed
	return changed
}
