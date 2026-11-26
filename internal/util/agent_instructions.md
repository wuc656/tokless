# Agent Instructions

Token-saving defaults (preserve normal reasoning and behavior):
- Surgical Coding (principles) — touch only requested lines; no unrequested test or mock files.
- Terse Output (caveman) — concise prose replies; exact code, paths, commands, and reasoning.
- Minimal Diff (ponytail) — reuse existing project code and stdlib before new abstractions.
- Code Index (codegraph) — `codegraph_explore` for symbol flows, callers, and blast radius.
- Context Tools (context-mode) — `ctx_*` to keep large logs, diffs, and command output in sandbox.

## Surgical Coding (principles)

- Touch only code required by the request; avoid unrelated refactors, reformatting, or speculative features.
- Verify with the project's existing build, typecheck, or test command when appropriate. Never create unrequested test files, mock harnesses, or standalone Python simulation scripts.

## Terse Output (caveman)

- Compress user-facing prose only; keep normal reasoning, tool decisions, code, comments, commits, and docs unchanged.
- Lead with the answer; omit pleasantries, filler, and tool-by-tool narration. Keep identifiers, paths, commands, errors, numbers, and negations exact in the user's language.

## Minimal Diff (ponytail)

- Prefer existing project helpers, then standard library, then current dependencies before adding new helpers or packages.
- Favor direct changes over new wrappers, config layers, or one-off abstractions unless requested.

## Code Index (codegraph)

- Prefer `codegraph_explore` first for architecture, call paths, callers, and blast radius when `.codegraph/` is indexed; read specific files directly whenever exact lines or edits are needed.
- Skip `codegraph_explore` for configs, docs, tiny repos, or when unavailable; if results spill, query the needed symbol or filter via `context-mode` instead of reading the whole spill file.

## Context Tools (context-mode)

- Use host tools for edits, Git writes, small reads, and short commands (sandbox writes do not persist on host).
- Use `ctx_execute` (`file` param for large files/logs), `ctx_batch` (multi-command + search), `ctx_index`, and `ctx_search` whenever raw output would flood context; print only the distilled answer. Windows shell: `pwsh -NoProfile -Command`.
