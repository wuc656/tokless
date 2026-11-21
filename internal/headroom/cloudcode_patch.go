package headroom

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/util"
)

const (
	cloudCodePatchMarker           = "gemini:tool_schema_compaction"
	cloudCodeGeminiRoutingMarker   = "tokless:antigravity_routing_v1"
	cloudCodeGeminiNonStreamMarker = "tokless:cloudcode_generate_v1"
	cloudCodeRouteSpecsMarker      = "handle_google_cloudcode_generate"
	cloudCodeRuntimePatchMarker    = "CLOUDCODE_INTERNAL_SLASH_PREFIX"
	cloudCodeRoutesPatchMarker     = "tokless:cloudcode_routes_v3"
	cloudCodeStreamingPatchMarker  = "tokless:binary_streaming_passthrough_v1"
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

type patchReplacement struct {
	old []byte
	new []byte
}

func applyAtomicReplacements(path, marker string, replacements []patchReplacement) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(marker)) {
		return false, nil
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

func patchCloudCodeGemini(path string) (bool, error) {
	return applyAtomicReplacements(path, cloudCodePatchMarker, []patchReplacement{
		{[]byte("        contents = request_payload.get(\"contents\", [])\n        headers = dict(request.headers.items())\n"), []byte("        contents = request_payload.get(\"contents\", [])\n        tools_before = request_payload.get(\"tools\")\n        tools_after = tools_before\n        tools_modified = False\n        headers = dict(request.headers.items())\n")},
		{[]byte("        if not messages:\n            original_tokens = 0\n        optimized_messages = messages\n"), []byte("        if not messages:\n            original_tokens = 0\n        _tool_tokens_before = 0\n        _tool_tokens_after = 0\n        optimized_messages = messages\n")},
		{[]byte("            if not is_antigravity:\n                if optimized_system:\n                    request_payload[\"systemInstruction\"] = optimized_system\n                elif \"systemInstruction\" in request_payload:\n                    del request_payload[\"systemInstruction\"]\n\n        tokens_saved = original_tokens - optimized_tokens\n        optimization_latency = (time.time() - start_time) * 1000\n        base_url = self._resolve_cloudcode_base_url(is_antigravity)\n"), []byte("            if optimized_system:\n                request_payload[\"systemInstruction\"] = optimized_system\n\n        if not _decision.bypass_header_set and isinstance(tools_before, list) and tools_before:\n            try:\n                from headroom.proxy.tool_schema_compaction import compact_tools\n\n                compacted_payload, tools_modified, _, _ = compact_tools(\n                    {\"tools\": tools_before}\n                )\n                tools_after = compacted_payload.get(\"tools\", tools_before)\n                if tools_modified:\n                    request_payload[\"tools\"] = tools_after\n            except Exception:\n                tools_modified = False\n                tools_after = tools_before\n                logger.debug(\"Cloud Code tool schema compaction skipped\", exc_info=True)\n            if tools_modified:\n                try:\n                    _tool_tokens_before = tokenizer.count_text(\n                        json.dumps(tools_before, default=str)\n                    )\n                    _tool_tokens_after = tokenizer.count_text(\n                        json.dumps(tools_after, default=str)\n                    )\n                except Exception:\n                    logger.debug(\"Cloud Code tool token recount skipped\", exc_info=True)\n\n        if 0 < _tool_tokens_after < _tool_tokens_before:\n            original_tokens += _tool_tokens_before\n            optimized_tokens += _tool_tokens_after\n            transforms_applied.append(\"gemini:tool_schema_compaction\")\n        tokens_saved = max(0, original_tokens - optimized_tokens)\n        optimization_latency = (time.time() - start_time) * 1000\n        base_url = self._resolve_cloudcode_base_url(is_antigravity)\n")},
	})
}

func patchCloudCodeGeminiNonStream(path string) (bool, error) {
	oldHead := []byte("    async def handle_google_cloudcode_stream(\n        self,\n        request: Request,\n    ) -> StreamingResponse | JSONResponse:\n        \"\"\"Handle Pi/OpenClaw Google Cloud Code Assist and Antigravity streaming requests.\"\"\"\n        from fastapi import HTTPException\n        from fastapi.responses import JSONResponse\n\n        from headroom.proxy.helpers import _read_request_json\n        from headroom.utils import extract_user_query\n\n        start_time = time.time()\n        request_id = await self._next_request_id()\n\n        try:\n            body = await _read_request_json(request)\n")
	newHead := []byte("    async def handle_google_cloudcode_stream(\n        self,\n        request: Request,\n        *,\n        stream: bool = True,\n    ) -> Response | StreamingResponse | JSONResponse:\n        \"\"\"Handle Pi/OpenClaw Google Cloud Code Assist and Antigravity streaming requests.\"\"\"\n        from fastapi import HTTPException\n        from fastapi.responses import JSONResponse\n\n        from headroom.proxy.helpers import _read_request_json\n        from headroom.utils import extract_user_query\n\n        _req_ct = request.headers.get(\"content-type\", \"\").lower()\n        if _req_ct.startswith((\"application/connect+\", \"application/grpc\", \"application/proto\")):\n            from headroom.proxy.request_scope import normalize_request_path\n\n            normalize_request_path(\n                request,\n                \"/v1internal:streamGenerateContent\" if stream else \"/v1internal:generateContent\",\n            )\n            _is_ag = self._is_cloudcode_antigravity_request({}, dict(request.headers.items()))\n            _fallback_base = self._resolve_cloudcode_base_url(_is_ag)\n            if stream or _req_ct.startswith((\"application/connect+\", \"application/grpc\")):\n                return await self._handle_streaming_passthrough(\n                    request=request,\n                    base_url=_fallback_base,\n                    endpoint_name=\"cloudcode_stream\",\n                    provider=\"gemini\",\n                )\n            return await self.handle_passthrough(\n                request,\n                _fallback_base,\n                \"cloudcode_passthrough\",\n                \"gemini\",\n            )\n\n        start_time = time.time()\n        request_id = await self._next_request_id()\n\n        try:\n            body = await _read_request_json(request)\n")

	oldTail := []byte("        stream_url = f\"{base_url}/v1internal:streamGenerateContent\"\n        if request.url.query:\n            stream_url = f\"{stream_url}?{request.url.query}\"\n\n        return await self._stream_response(\n            stream_url,\n            headers,\n            body,\n            \"gemini\",\n            model,\n            request_id,\n            original_tokens,\n            optimized_tokens,\n            tokens_saved,\n            transforms_applied,\n            tags,\n            optimization_latency,\n            prefix_tracker=gemini_prefix_tracker,\n            original_messages=messages,\n        )\n")
	newTail := []byte("        _endpoint_path = (\n            \"/v1internal:streamGenerateContent\" if stream else \"/v1internal:generateContent\"\n        )\n        stream_url = f\"{base_url}{_endpoint_path}\"\n        if request.url.query:\n            stream_url = f\"{stream_url}?{request.url.query}\"\n\n        if stream or request.query_params.get(\"alt\", \"\").lower() == \"sse\":\n            return await self._stream_response(\n                stream_url,\n                headers,\n                body,\n                \"gemini\",\n                model,\n                request_id,\n                original_tokens,\n                optimized_tokens,\n                tokens_saved,\n                transforms_applied,\n                tags,\n                optimization_latency,\n                prefix_tracker=gemini_prefix_tracker,\n                original_messages=messages,\n            )\n\n        from fastapi.responses import Response\n\n        for _hop_key in (\n            \"connection\",\n            \"keep-alive\",\n            \"proxy-connection\",\n            \"transfer-encoding\",\n            \"upgrade\",\n            \"te\",\n        ):\n            headers.pop(_hop_key, None)\n        client = classify_client(headers)\n        try:\n            response = await self._retry_request(\"POST\", stream_url, headers, body)\n        except Exception as e:\n            await self.metrics.record_failed(provider=\"gemini\")\n            log_request_failure(\n                request_id, \"Cloud Code Assist request\", e, provider=\"gemini\", model=model\n            )\n            return JSONResponse(\n                status_code=502,\n                content={\n                    \"error\": {\n                        \"message\": \"An error occurred while processing your request. Please try again.\",\n                        \"code\": 502,\n                    }\n                },\n            )\n\n        total_latency = (time.time() - start_time) * 1000\n        response_content = response.content\n        resp_json: dict[str, Any] | None = None\n        total_input_tokens = optimized_tokens\n        provider_prompt_tokens = 0\n        output_tokens = 0\n        cache_read_tokens = 0\n        try:\n            parsed = response.json()\n            if isinstance(parsed, dict):\n                resp_json = parsed\n                inner = (\n                    parsed.get(\"response\")\n                    if isinstance(parsed.get(\"response\"), dict)\n                    else parsed\n                )\n                usage = inner.get(\"usageMetadata\", {}) if isinstance(inner, dict) else {}\n                if not isinstance(usage, dict):\n                    usage = {}\n                if usage.get(\"promptTokenCount\") is not None:\n                    total_input_tokens = int(usage[\"promptTokenCount\"])\n                    provider_prompt_tokens = total_input_tokens\n                output_tokens = gemini_output_tokens(usage)\n                cache_read_tokens = _usage_int(usage.get(\"cachedContentTokenCount\"))\n        except (json.JSONDecodeError, ValueError, KeyError, TypeError, AttributeError) as e:\n            logger.debug(\n                f\"[{request_id}] Failed to extract usage from Cloud Code response: {e}\"\n            )\n\n        uncached_input_tokens = max(0, total_input_tokens - cache_read_tokens)\n        if response.status_code == 200 and isinstance(resp_json, dict):\n            gemini_prefix_tracker.update_from_response(\n                cache_read_tokens=cache_read_tokens,\n                cache_write_tokens=max(total_input_tokens - cache_read_tokens, 0),\n                messages=optimized_messages,\n                original_messages=messages,\n            )\n\n        effective_original_tokens = (\n            max(original_tokens, total_input_tokens + tokens_saved)\n            if total_input_tokens > 0\n            else original_tokens\n        )\n        await self._record_request_outcome(\n            RequestOutcome(\n                request_id=request_id,\n                provider=\"gemini\",\n                model=model,\n                status_code=response.status_code,\n                original_tokens=effective_original_tokens,\n                optimized_tokens=total_input_tokens,\n                provider_input_tokens=provider_prompt_tokens,\n                output_tokens=output_tokens,\n                tokens_saved=tokens_saved,\n                attempted_input_tokens=total_input_tokens + tokens_saved,\n                cache_read_tokens=cache_read_tokens,\n                uncached_input_tokens=uncached_input_tokens,\n                total_latency_ms=total_latency,\n                overhead_ms=optimization_latency,\n                transforms_applied=tuple(transforms_applied),\n                num_messages=len(contents) if isinstance(contents, list) else 0,\n                tags=tags or {},\n                client=client,\n            )\n        )\n\n        response_headers = sanitize_forwarded_response_headers(response.headers)\n        response_headers[\"x-headroom-tokens-before\"] = str(original_tokens)\n        response_headers[\"x-headroom-tokens-after\"] = str(optimized_tokens)\n        response_headers[\"x-headroom-tokens-saved\"] = str(tokens_saved)\n        response_headers[\"x-headroom-model\"] = model\n        if transforms_applied:\n            from headroom.proxy.cost import header_safe_transforms\n\n            response_headers[\"x-headroom-transforms\"] = \",\".join(\n                header_safe_transforms(transforms_applied)\n            )\n        if cache_read_tokens > 0:\n            response_headers[\"x-headroom-cached\"] = \"true\"\n\n        return Response(\n            content=response_content,\n            status_code=response.status_code,\n            headers=response_headers,\n        )\n\n    async def handle_google_cloudcode_generate(\n        self,\n        request: Request,\n    ) -> Response | StreamingResponse | JSONResponse:\n        \"\"\"Handle Google Cloud Code Assist and Antigravity non-streaming generateContent requests (" + cloudCodeGeminiNonStreamMarker + ").\"\"\"\n        return await self.handle_google_cloudcode_stream(request, stream=False)\n")

	return applyAtomicReplacements(path, cloudCodeGeminiNonStreamMarker, []patchReplacement{
		{oldHead, newHead},
		{oldTail, newTail},
	})
}

func patchCloudCodeRouteSpecs(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if bytes.Contains(data, []byte(cloudCodeRouteSpecsMarker)) {
		return false, nil
	}

	// Ensure route_specs.py never registers handle_google_cloudcode_generate unless sibling gemini.py defines it.
	geminiPath := filepath.Join(filepath.Dir(filepath.Dir(path)), "proxy", "handlers", "gemini.py")
	geminiData, err := os.ReadFile(geminiPath)
	if err != nil || !bytes.Contains(geminiData, []byte("async def handle_google_cloudcode_generate(")) {
		return false, nil
	}

	old := []byte("CLOUDCODE_HANDLER_ROUTES: tuple[ProviderHandlerRoute, ...] = (\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1internal:streamGenerateContent\",\n        \"handle_google_cloudcode_stream\",\n    ),\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1/v1internal:streamGenerateContent\",\n        \"handle_google_cloudcode_stream\",\n    ),\n)\n")
	new := []byte("CLOUDCODE_HANDLER_ROUTES: tuple[ProviderHandlerRoute, ...] = (\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1internal:streamGenerateContent\",\n        \"handle_google_cloudcode_stream\",\n    ),\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1/v1internal:streamGenerateContent\",\n        \"handle_google_cloudcode_stream\",\n    ),\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1internal:generateContent\",\n        \"" + cloudCodeRouteSpecsMarker + "\",\n    ),\n    ProviderHandlerRoute(\n        \"POST\",\n        \"/v1/v1internal:generateContent\",\n        \"" + cloudCodeRouteSpecsMarker + "\",\n    ),\n)\n")

	if bytes.Count(data, old) != 1 {
		return false, nil
	}
	return writePatchedFile(path, bytes.Replace(data, old, new, 1))
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
	oldV2 := []byte("        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            # tokless:cloudcode_routes_v2\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            _cc_ua = request.headers.get(\"user-agent\", \"\").lower()\n            _cc_api = request.headers.get(\"x-goog-api-client\", \"\").lower()\n            _cc_client = request.headers.get(\"x-client\", \"\").lower()\n            _cc_antigravity = (\n                \"antigravity\" in _cc_ua\n                or \"jetski\" in _cc_ua\n                or \"antigravity\" in _cc_api\n                or \"jetski\" in _cc_api\n                or _cc_client == \"antigravity\"\n                or normalized_cloudcode_path.startswith(\"/google.internal.cloud.code.\")\n            )\n            if _cc_antigravity and _cloudcode_target.rstrip(\"/\") == \"https://cloudcode-pa.googleapis.com\":\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n            _cc_stream = (\n                \":stream\" in normalized_cloudcode_path.lower()\n                or request.query_params.get(\"alt\", \"\").lower() == \"sse\"\n                or \"text/event-stream\" in request.headers.get(\"accept\", \"\").lower()\n            )\n            if _cc_stream:\n                return await proxy._handle_streaming_passthrough(\n                    request=request,\n                    base_url=_cloudcode_target,\n                    endpoint_name=\"cloudcode_stream\",\n                    provider=\"gemini\",\n                )\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n                \"cloudcode_passthrough\",\n                \"gemini\",\n            )\n")
	new := []byte("        normalized_cloudcode_path = normalize_cloudcode_passthrough_path(path)\n        # " + cloudCodeRoutesPatchMarker + "\n        _cc_ct = request.headers.get(\"content-type\", \"\").lower()\n        _cc_is_connect_or_grpc = (\n            _cc_ct.startswith((\"application/connect+\", \"application/grpc\", \"application/proto\"))\n            or \"connect-protocol-version\" in request.headers\n            or request.query_params.get(\"connect\", \"\").lower() == \"v1\"\n        )\n        _cc_clean_path = path.lstrip(\"/\")\n        if normalized_cloudcode_path is None:\n            if _cc_clean_path.startswith(\"google.cloud.cloudaicompanion.\") or (\n                _cc_is_connect_or_grpc and not request.headers.get(\"x-goog-api-key\")\n            ):\n                normalized_cloudcode_path = f\"/{_cc_clean_path}\"\n        if normalized_cloudcode_path is not None:\n            normalize_request_path(request, normalized_cloudcode_path)\n            _cloudcode_target = _api_target(proxy, \"cloudcode\")\n            _cc_ua = request.headers.get(\"user-agent\", \"\").lower()\n            _cc_api = request.headers.get(\"x-goog-api-client\", \"\").lower()\n            _cc_client = request.headers.get(\"x-client\", \"\").lower()\n            _cc_antigravity = (\n                \"antigravity\" in _cc_ua\n                or \"jetski\" in _cc_ua\n                or \"antigravity\" in _cc_api\n                or \"jetski\" in _cc_api\n                or _cc_client == \"antigravity\"\n                or normalized_cloudcode_path.startswith(\"/google.internal.cloud.code.\")\n            )\n            if _cc_antigravity and _cloudcode_target.rstrip(\"/\") == \"https://cloudcode-pa.googleapis.com\":\n                _cloudcode_target = \"https://daily-cloudcode-pa.sandbox.googleapis.com\"\n            _cc_path_lower = normalized_cloudcode_path.lower()\n            _cc_accept = request.headers.get(\"accept\", \"\").lower()\n            _cc_stream = (\n                \":stream\" in _cc_path_lower\n                or \"/stream\" in _cc_path_lower\n                or request.query_params.get(\"alt\", \"\").lower() == \"sse\"\n                or \"text/event-stream\" in _cc_accept\n                or \"application/x-ndjson\" in _cc_accept\n                or _cc_ct.startswith((\"application/connect+\", \"application/grpc\"))\n            )\n            if _cc_stream:\n                return await proxy._handle_streaming_passthrough(\n                    request=request,\n                    base_url=_cloudcode_target,\n                    endpoint_name=\"cloudcode_stream\",\n                    provider=\"gemini\",\n                )\n\n            return await proxy.handle_passthrough(\n                request,\n                _cloudcode_target,\n                \"cloudcode_passthrough\",\n                \"gemini\",\n            )\n")

	var target []byte
	switch {
	case bytes.Count(data, oldOriginal) == 1:
		target = oldOriginal
	case bytes.Count(data, oldV1) == 1:
		target = oldV1
	case bytes.Count(data, oldV2) == 1:
		target = oldV2
	default:
		return false, nil
	}
	return writePatchedFile(path, bytes.Replace(data, target, new, 1))
}

func patchCloudCodeStreamingPassthrough(path string) (bool, error) {
	hopStrip := "        headers = dict(request.headers.items())\n        for _hop_key in (\n            \"host\",\n            \"accept-encoding\",\n            \"connection\",\n            \"keep-alive\",\n            \"proxy-connection\",\n            \"transfer-encoding\",\n            \"upgrade\",\n            \"content-length\",\n        ):\n            headers.pop(_hop_key, None)\n        if headers.get(\"te\", \"\").lower() != \"trailers\":\n            headers.pop(\"te\", None)\n"

	oldPtHeaders := []byte("        headers = dict(request.headers.items())\n        headers.pop(\"host\", None)\n        headers.pop(\"accept-encoding\", None)\n        client = classify_client(headers)\n        tags = extract_tags(headers)\n        # PR-A5 (P5-49): strip internal x-headroom-* before forwarding upstream.\n")
	newPtHeaders := []byte(hopStrip + "        client = classify_client(headers)\n        tags = extract_tags(headers)\n        # PR-A5 (P5-49): strip internal x-headroom-* before forwarding upstream.\n")

	oldStreamHeaders := []byte("        headers = dict(request.headers.items())\n        headers.pop(\"host\", None)\n        headers.pop(\"accept-encoding\", None)\n        client = classify_client(headers)\n        tags = extract_tags(headers)\n        if _upstream_response is not None:\n")
	newStreamHeaders := []byte(hopStrip + "        client = classify_client(headers)\n        tags = extract_tags(headers)\n        if _upstream_response is not None:\n")

	oldGen := []byte("        async def generate():\n            try:\n                async with contextlib.aclosing(upstream_response) as response:\n                    async for chunk in response.aiter_bytes():\n                        if stream_state[\"ttfb_ms\"] is None:\n                            stream_state[\"ttfb_ms\"] = (time.time() - start_time) * 1000\n                        stream_state[\"total_bytes\"] += len(chunk)\n                        stream_state[\"sse_buffer\"].extend(chunk)\n                        if len(stream_state[\"sse_buffer\"]) > MAX_SSE_BUFFER_SIZE:\n                            tail = bytes(stream_state[\"sse_buffer\"][-MAX_SSE_BUFFER_SIZE // 2 :])\n                            stream_state[\"sse_buffer\"] = bytearray(tail)\n\n                        _absorb_usage(\n                            self._parse_sse_usage_from_buffer(stream_state, stream_provider)\n                        )\n                        yield chunk\n            finally:\n                completed_normally = True\n                buf = stream_state[\"sse_buffer\"]\n                if len(buf) > 0:\n                    buf.extend(b\"\\n\\n\")\n                    # A truncated stream can end inside a multi-byte character;\n                    # the strict decoder then raises and would skip the outcome\n                    # below. Keep the usage already parsed, drop the unusable\n                    # tail, and report the stream as failed.\n                    try:\n                        _absorb_usage(\n                            self._parse_sse_usage_from_buffer(stream_state, stream_provider)\n                        )\n                    except UnicodeDecodeError:\n                        completed_normally = False\n")
	newGen := []byte("        # " + cloudCodeStreamingPatchMarker + "\n        _resp_ct = upstream_response.headers.get(\"content-type\", \"\").lower()\n        _is_sse_resp = _resp_ct.startswith(\"text/event-stream\")\n\n        async def generate():\n            _stream_failed = False\n            try:\n                async with contextlib.aclosing(upstream_response) as response:\n                    async for chunk in response.aiter_bytes():\n                        if stream_state[\"ttfb_ms\"] is None:\n                            stream_state[\"ttfb_ms\"] = (time.time() - start_time) * 1000\n                        stream_state[\"total_bytes\"] += len(chunk)\n                        if _is_sse_resp:\n                            stream_state[\"sse_buffer\"].extend(chunk)\n                            if len(stream_state[\"sse_buffer\"]) > MAX_SSE_BUFFER_SIZE:\n                                tail = bytes(stream_state[\"sse_buffer\"][-MAX_SSE_BUFFER_SIZE // 2 :])\n                                stream_state[\"sse_buffer\"] = bytearray(tail)\n\n                            _absorb_usage(\n                                self._parse_sse_usage_from_buffer(stream_state, stream_provider)\n                            )\n                        yield chunk\n            except Exception:\n                _stream_failed = True\n                raise\n            finally:\n                completed_normally = not _stream_failed\n                buf = stream_state[\"sse_buffer\"]\n                if _is_sse_resp and len(buf) > 0:\n                    buf.extend(b\"\\n\\n\")\n                    # A truncated stream can end inside a multi-byte character;\n                    # the strict decoder then raises and would skip the outcome\n                    # below. Keep the usage already parsed, drop the unusable\n                    # tail, and report the stream as failed.\n                    try:\n                        _absorb_usage(\n                            self._parse_sse_usage_from_buffer(stream_state, stream_provider)\n                        )\n                    except UnicodeDecodeError:\n                        completed_normally = False\n")

	return applyAtomicReplacements(path, cloudCodeStreamingPatchMarker, []patchReplacement{
		{oldPtHeaders, newPtHeaders},
		{oldStreamHeaders, newStreamHeaders},
		{oldGen, newGen},
	})
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
	if util.IsRunningInTest() && !util.HasHomeOverride() {
		return false
	}
	// gemini.py must run before route_specs.py so handle_google_cloudcode_generate exists first.
	changed := applyPatchToRoots(
		cloudCodePatchRoots(),
		"Cloud Code",
		patchCloudCodeGemini,
		patchCloudCodeGeminiRouting,
		patchCloudCodeGeminiNonStream,
	)
	changed = applyPatchToRoots(
		cloudCodePatchRootsFor("headroom", "providers", "route_specs.py"),
		"Cloud Code route specs",
		patchCloudCodeRouteSpecs,
	) || changed
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
	changed = applyPatchToRoots(
		cloudCodePatchRootsFor("headroom", "proxy", "handlers", "openai.py"),
		"Cloud Code streaming",
		patchCloudCodeStreamingPassthrough,
	) || changed
	return changed
}

// CloudCodePatchStatus inspects installed Headroom site-packages without mutating them
// and reports how many expected Cloud Code / Antigravity patch markers are present.
func CloudCodePatchStatus() (applied, total int) {
	if util.IsRunningInTest() && !util.HasHomeOverride() {
		return 0, 0
	}
	checks := []struct {
		patterns []string
		markers  []string
	}{
		{
			patterns: cloudCodePatchRoots(),
			markers:  []string{cloudCodePatchMarker, cloudCodeGeminiRoutingMarker, cloudCodeGeminiNonStreamMarker},
		},
		{
			patterns: cloudCodePatchRootsFor("headroom", "providers", "route_specs.py"),
			markers:  []string{cloudCodeRouteSpecsMarker},
		},
		{
			patterns: cloudCodePatchRootsFor("headroom", "providers", "cloudcode", "runtime.py"),
			markers:  []string{cloudCodeRuntimePatchMarker},
		},
		{
			patterns: cloudCodeRoutesPatchRoots(),
			markers:  []string{cloudCodeRoutesPatchMarker},
		},
		{
			patterns: cloudCodePatchRootsFor("headroom", "proxy", "handlers", "openai.py"),
			markers:  []string{cloudCodeStreamingPatchMarker},
		},
	}

	for _, c := range checks {
		seen := map[string]bool{}
		for _, pattern := range c.patterns {
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
				data, err := os.ReadFile(clean)
				if err != nil {
					continue
				}
				for _, m := range c.markers {
					total++
					if bytes.Contains(data, []byte(m)) {
						applied++
					}
				}
			}
		}
	}
	return applied, total
}
