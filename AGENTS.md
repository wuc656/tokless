# Agent Guidelines for tokless

## Communication
- **Language**: Always respond to the user in Traditional Chinese (繁體中文). Keep prose concise and technical. Internal thinking, codebase docs, and commits remain in English for token efficiency.

## Critical Constraints (Host Isolation)
- **NEVER disrupt host proxy**: Do NOT stop or restart the local proxy on port 8787. Never execute `tokless proxy down` or kill active proxy processes.
- **NEVER mutate host installation**: Do NOT modify files under `AppData\Local\tokless` or `AppData\Local\Programs\tokless`. No self-updates or installers on host.
- **Repo-only scope**: All changes, builds, and tests must remain strictly within this repository (`C:\Users\wuc656\Documents\tokless`).

## Engineering Principles
- **Simplicity First**: Minimal code to solve the issue. No speculative abstractions or unrequested features.
- **Surgical Edits**: Touch only what is necessary. Match existing style. Clean only changes made by yourself.
- **Goal-Driven Verification**: Verify with `go build ./...` and `go test` before reporting complete.

## Build & Test Workflow
- Build: `go build ./...`
- Release executable: `go build -ldflags="-s -w" -o dist/release/tokless-windows-x64.exe ./cmd/tokless`
- Unit tests: `go test ./internal/headroom ./internal/util ./cmd/...`

## Tools
### Code Index (codegraph)
- Query `codegraph_explore` for architecture, call graph, and symbol dependencies.

### Context Tools (context-mode)
- Use `ctx_execute` / `ctx_batch_execute` for large file parsing and sandbox analysis to minimize context usage.
- Windows execution: `pwsh -NoProfile -Command`, absolute paths.
