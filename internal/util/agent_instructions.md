# Agent Instructions

Apply on every coding task:

**Tools (call):**
- Code Index (codegraph) — MCP `codegraph_explore` for structure, flows, dependencies.
- Context Tools (context-mode) — MCP `ctx_*` for large, uncertain, multi-source analysis.

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

MCP tools `ctx_*` keep large or uncertain analysis out of context.

### Rules

| Tool | Role | Replaces |
|------|------|----------|
| `ctx_execute` | Run code in sandbox. Only stdout enters context. | Bash for analysis tasks |
| `ctx_execute_file` | Process file in sandbox. Raw bytes never leave. | Read on large files (>200 lines) |
| `ctx_batch_execute` | Run N commands + auto-index output. Search in same call. Concurrency 1-8. | Multiple Bash + grep |
| `ctx_index` | Chunk markdown/text into FTS5. Queryable via `ctx_search`. | Manual grep over pasted content |
| `ctx_search` | Multi-strategy search across indexed content + session memory. Typo correction. | Re-asking user, re-deriving |
| `ctx_fetch_and_index` | Fetch URL, chunk and index. Cache 24h; `ttl: 0` or `force: true` bypasses cache. Supports `requests` + `concurrency: 1-8`. | Webfetch |

### Examples

```shell
ctx_execute(language:"shell", code:"git log --oneline -20 | wc -l")
```

```javascript
ctx_execute(language:"javascript", code:`
  const fs = require('node:fs');
  const files = fs.readdirSync('src').filter(f => f.endsWith('.ts'));
  const counts = files.map(f => ({
    file: f,
    lines: fs.readFileSync('src/' + f, 'utf8').split('\n').length
  }));
  console.log(JSON.stringify(counts));
`)
```

```javascript
ctx_execute_file(path:"app.log", language:"javascript", code:`
  const lines = FILE_CONTENT.split('\n');
  const errs = lines.filter(l => /ERROR|FATAL/.test(l));
  console.log(errs.length + ' errors');
`)
```

```javascript
ctx_batch_execute(commands:[
  {label:"diff", command:"git diff HEAD~1"},
  {label:"status", command:"git status"},
], queries:["failures","errors"], timeout:120000, concurrency:2,
 cwd:".", query_scope:"batch")
```

```javascript
ctx_fetch_and_index(requests:[
  {url:"https://docs.example.com/api", source:"api-docs"},
  {url:"https://docs.example.com/guide", source:"guide"},
], concurrency:4)
ctx_search(queries:["auth endpoint","rate limits"], source:"api-docs")
```

Use host tools for edits, Git writes, navigation, installs, and small output. Use context-mode for analysis and large command output. Sandbox writes do not affect host files.

Windows: `pwsh -NoProfile -Command`, absolute paths, `X:\` maps to `/x/`, quote spaces.
