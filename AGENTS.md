# Agent Guidelines

## Communication
- Reply to user in Traditional Chinese (繁體中文): terse, technical.
- Keep thinking, docs, comments, and commits in English to save tokens.

## Architecture & Scope
- `cmd/tokless/`: CLI entrypoint, arg parsing, daemon flags (`__proxy-watch`, `__proxy-run`), console hiding (`toklessw.exe` built with `-H windowsgui` on Windows).
- `internal/core/`: Central `AgentManifest` and `ToolManifest` registries (no per-agent/tool branching in command handlers).
- `internal/tools/`: Active tools (`rtk`, `headroom`, `codegraph`, `context-mode`). Note: `caveman`, `ponytail`, `principles`, and `projectmem` have been removed.
  - `codegraph`: Auto-initializes missing indexes and runs incremental `codegraph sync --quiet` when `pendingChanges` or `filesNeedingReindex` exist on healthy indexes.
- `internal/agents/`: Per-harness wiring, MCP adapters, shims, and BYOK routing across 16 agents (Claude, Codex, Antigravity, Copilot, Cursor, OpenCode, Cline, Droid, Grok, Kilo, Kimi, Pi, OMP, Amp, ZCode, DeepSeek).
- `internal/headroom/`: Local proxy lifecycle (port 8787), BYOK gateway (port 18787), supervisor/watchdog, OS autostart (`schtasks` / `launchd` / `systemd`), and 7 idempotent Cloud Code / Antigravity runtime patches (`cloudcode_patch.go`: tool schema compaction, non-streaming `generateContent` compression, Antigravity daily sandbox routing, `/v1internal/*` slash normalization, Connect-Web/gRPC-Web/Protobuf binary streaming passthrough across `%APPDATA%\uv\tools` and `%LOCALAPPDATA%`).
- `internal/commands/`: CLI commands (`init`, `doctor`, `update`, `disable`, `index`, `proxy`, `run-mcp`, `selfupdate`, `rtk-hook`).
  - `run-mcp`: Bounded MCP proxy for `context-mode` (exposes 4 core tools: `ctx_execute`, `ctx_batch`, `ctx_index`, `ctx_search`) and background CodeGraph/Headroom bootstrap.
  - `rtk_hook.go`: Distinguishes `||` from `|`, and unwraps single-command Windows `pwsh`/`powershell -Command` and `cmd /c` wrappers for RTK output compression.
  - `doctor_stats.go`: Queries Headroom savings/perf, RTK gain, CodeGraph status, and Context-Mode v2 `~/.context-mode/projects/*/context.db` (`node:sqlite` read-only stats).
  - `selfupdate.go`: Atomic binary swap + Windows companion `toklessw.exe` sync.
- `internal/util/`: Config mutation (idempotent byte-stable JSONC/TOML preserving comments), process execution, path resolution, unified instruction blocks.

## Host Isolation (Critical)
- **No proxy disruption**: Never stop, restart, or kill the active proxy on port 8787 (`tokless proxy down`).
- **No host mutation**: Never modify host installation directories (`%LOCALAPPDATA%\tokless`, `%LOCALAPPDATA%\Programs\tokless`, `%APPDATA%\uv\tools`, `%APPDATA%\npm`), Windows Registry (`HKCU\Environment`), or live user harness configs (`~/.codex`, `~/.gemini`, `%APPDATA%\Antigravity IDE`, etc.).
- **Test isolation**: Always use `t.TempDir()`, `util.SetHomeOverride()`, and `TOKLESS_TEST=1` (with `util.IsRunningInTest()` / `util.HasHomeOverride()` guards) when testing config mutations, runtime patches, or OS services/tasks.

## Config & Platform Rules
- **Idempotency**: Config writes must be byte-stable, preserve user key ordering, and retain JSONC/TOML comments via `internal/util` helpers.
- **Windows compatibility**:
  - `scripts/install.ps1` must remain strictly ASCII-only (PowerShell 5.1 safe).
  - Background spawns must set `CREATE_NO_WINDOW` (`0x08000000`) / `HideWindow: true`.
  - Task Scheduler (`schtasks`) output parsing must remain locale-independent (handle localized Windows labels and CP950/UTF-8 decoding).
  - Use single-line commands in shell tool calls so the host RTK `PreToolUse` hook does not fail on multiline PowerShell here-strings.

## Workflow & Git
- Build & Vet: `go build ./...` && `go vet ./...`
- Release: `& "C:\Program Files\Git\bin\bash.exe" scripts/build-release.sh <tag>` (builds all 6 binaries including `toklessw-windows-x64.exe` and `SHA256SUMS`)
- Test: `go test ./...`
- Git: All commits and tags must be signed (`git commit -S`, `git tag -s`) using the environment's configured signing method.
