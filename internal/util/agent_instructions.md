# Agent Instructions

Apply on every coding task:

**Tools (call):**
- Code Index (codegraph) — MCP `codegraph_explore` for structure, flows, dependencies.
- Context Tools (context-mode) — `ctx_*` for large, uncertain, multi-source analysis.

## Code Index (codegraph)

MCP tool `codegraph_explore` gives source, call path, and blast radius in one call.

### Rules

- `.codegraph/` index exists: use CodeGraph first.
  - Use for: how X works, flows, architecture, callers, blast radius, symbols.
  - Trust results — no re-read/re-grep. Stale banner? Read only listed files.
  - Result spilled? Search only needed symbol; never read whole spill. Use context-mode when spill is large.
  - Configs/docs/.env/non-indexed: use normal tools.
- No `.codegraph/`: run `git ls-files | wc -l`.
  - ≤5 files or no git: use normal tools.
  - Otherwise: run `tokless index` once.
    - Ready: use CodeGraph.
    - Fails/CLI missing: use normal tools; do not retry.

### Examples

- `codegraph_explore("how does auth middleware validate a JWT")`
- `codegraph_explore("flow from HTTP request to DB query")`
- `codegraph_explore("OrderService.createOrder callers and blast radius")`

## Context Tools (context-mode)

Tools `ctx_*` keep large or uncertain analysis out of context.

### Rules

| Tool | Role |
|------|------|
| `ctx_execute` | Run code or a command in sandbox, or analyze a file via `file`. Only stdout enters context. |
| `ctx_batch` | Run many commands plus search `queries` in one call. |
| `ctx_index` | Index a file, text, or URL (`path`, `content`, `url`) with a `source` label. |
| `ctx_search` | Query indexed content and session memory. |

- `read` small files and files you will edit. Analyze large files and logs with `ctx_execute` (`file` param); print only the answer.

### Examples

```javascript
ctx_execute(language:"shell", code:"git log --oneline -20 | wc -l")
ctx_execute(language:"shell", file:"app.log", code:"grep -c ERROR \"$CTX_FILE\"")
ctx_batch(commands:["git diff HEAD~1"], queries:["failures"])
ctx_index(url:"https://docs.example.com/api", source:"api-docs")
ctx_search(queries:["auth endpoint"], source:"api-docs")
```

Use host tools for edits, Git writes, navigation, installs, and small output. Use context-mode for analysis and large command output. Sandbox writes do not affect host files.

Windows: `pwsh -NoProfile -Command`, absolute paths, `X:\` maps to `/x/`, quote spaces.
