# Agent Guidelines

## Communication
- Reply to user in Traditional Chinese (繁體中文): terse, technical.
- Keep thinking, docs, comments, and commits in English to save tokens.

## Architecture & Scope
- `cmd/tokless/`: CLI entrypoint, arg parsing, daemon flags (`__proxy-watch`, `__proxy-run`), console hiding.
- `internal/core/`: Central `AgentManifest` and `ToolManifest` registries (no per-agent/tool branching in command handlers).
- `internal/tools/`: Active tools (`rtk`, `headroom`, `codegraph`, `context-mode`). Note: `caveman`, `ponytail`, `principles`, and `projectmem` have been removed.
- `internal/agents/`: Per-harness wiring, MCP adapters, shims, and BYOK routing across 16 agents (Claude, Codex, Antigravity, Copilot, Cursor, OpenCode, Cline, Droid, Grok, Kilo, Kimi, Pi, OMP, Amp, ZCode, DeepSeek).
- `internal/headroom/`: Local proxy lifecycle (port 8787), BYOK gateway (port 18787), supervisor/watchdog, and OS autostart (`schtasks` / `launchd` / `systemd`).
- `internal/commands/`: CLI commands (`init`, `doctor`, `update`, `disable`, `index`, `proxy`, `run-mcp`, `selfupdate`).
- `internal/util/`: Config mutation (idempotent byte-stable JSONC/TOML), process execution, path resolution, unified instruction blocks.

## Host Isolation (Critical)
- **No proxy disruption**: Never stop, restart, or kill the active proxy on port 8787 (`tokless proxy down`).
- **No host mutation**: Never modify host installation directories (`%LOCALAPPDATA%\tokless`, `%LOCALAPPDATA%\Programs\tokless`) or live user harness configs (`~/.codex`, `~/.gemini`, etc.).
- **Test isolation**: Always use `t.TempDir()`, `util.SetHomeOverride()`, and `TOKLESS_TEST=1` when testing config mutations or OS services/tasks.

## Config & Platform Rules
- **Idempotency**: Config writes must be byte-stable and preserve user key ordering via `internal/util` JSONC/TOML helpers.
- **Windows compatibility**: `scripts/install.ps1` must remain strictly ASCII-only (PowerShell 5.1 safe). Background spawns must set `CREATE_NO_WINDOW` (`0x08000000`) / `HideWindow: true`.

## Workflow & Git
- Build & Vet: `go build ./...` && `go vet ./...`
- Release: `bash scripts/build-release.sh <tag>` (or `go build -trimpath -ldflags="-s -w -X github.com/HoangP8/tokless/internal/util.Version=<tag>" -o dist/release/tokless-windows-x64.exe ./cmd/tokless`)
- Test: `go test ./...`
- Git: All commits and tags must be signed (`git commit -S`, `git tag -s`) using the environment's configured signing method.
