# Web UI architecture

The UI planned for v0.5.0 and v0.6.0. This document is the structure those
releases build against; issues reference it rather than re-decide it.

Status: **planned**, nothing here is implemented yet.

## What this adds

`tada ui` starts a loopback HTTP server that serves a single-page interface from
assets embedded in the binary, backed by the same `store.Store` the CLI writes
through. The browser is a second client of the same JSON file, not a second
database.

## Constraints

These are the reason for most of the decisions below.

1. **One artifact.** Everything ships inside the `tada` binary built by
   GoReleaser today, for darwin, linux and windows on amd64 and arm64.
2. **No runtime dependency.** No Node, no Python, no browser extension, no
   daemon to install. Downloading the archive and running the executable is the
   whole installation.
3. **No build step for the UI.** The front end is hand-written HTML, CSS and
   native ES modules embedded with `//go:embed`. `go build ./cmd/tada` produces
   a runnable app; nothing is generated, so nothing can ship stale.
4. **Works offline.** No CDN, no remote fonts, no analytics, no network calls
   other than the loopback server. A test enforces it.
5. **No new Go dependencies.** `net/http` and the standard library only. The
   module currently has one direct dependency and this does not add a second.
6. **Portable data.** `tasks.json` and `schema_version` are unchanged. The UI
   adds no fields and needs no migration, so v0.4.x binaries keep working
   against a file the UI has written.

## Module map

```
cmd/ui.go            flags, startup line, browser hand-off, signal handling
ui/server.go         http.Server lifecycle, routing, middleware chain
ui/assets.go         //go:embed assets, content types, SPA fallback
ui/api.go            JSON handlers over store.Store
ui/assets/           the front end (no build step)
query/               filters and sorting shared by cmd and ui
model/               unchanged domain rules: NewTask, ParseDueDate
store/               unchanged persistence, including the inter-process lock
```

What each piece must not do:

| Package | Owns | Must not |
| --- | --- | --- |
| `cmd/ui.go` | Flag parsing, deciding the address, opening a browser, printing the URL, exiting on interrupt | Contain HTTP handlers or markup |
| `ui/server.go` | Listener, routes, middleware, graceful shutdown | Know about Cobra or `os.Args` |
| `ui/api.go` | Request/response shapes, status codes, validation order | Hold task state in memory or implement its own locking |
| `ui/assets.go` | Embedded FS, content types, caching headers | Know any endpoint paths |
| `ui/assets/` | Rendering, interaction, client-side view state | Be the only place filter semantics are implemented |
| `query/` | Filter and sort semantics | Touch the filesystem, flags or HTTP |

`query/` exists because filtering currently lives in `cmd/filter.go`, where the
CLI can use it and the server cannot. The CLI keeps its behaviour byte for byte;
the code moves, the semantics do not.

## Data flow

```
browser  ──HTTP──▶  ui/api.go  ──▶  store.Store  ──▶  tasks.json
   ▲                    │                                  │
   └──── JSON ──────────┘                                  │
                                                           │
tada add / tada edit ──▶ store.Store ──────────────────────┘
```

- The **file is the only source of truth**. Neither the server nor the page
  caches tasks between requests: a `GET` after an external `tada add` sees the
  new task, because the store rereads the file on every call.
- **Writes go through the existing lock.** `Add`, `Update` and `Delete` already
  take an exclusive lock for the whole read-modify-write, so a browser save and
  a terminal command can interleave without losing tasks. The API adds no second
  locking mechanism, and holding a lock across HTTP is forbidden.
- **The server is stateless.** Restarting it loses nothing, and two `tada ui`
  instances pointed at one file are safe — they contend for the lock like any
  other pair of processes.
- **The client holds only view state**: the current filter, sort, selection and
  in-flight optimistic updates. Task data is refetched on focus, on
  `visibilitychange`, and after every successful write.

## HTTP API

All routes are under `/api/`. Task objects use the data file's field names
(`id`, `title`, `completed`, `priority`, `due_date`), so there is one vocabulary
rather than a translation layer.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness for tests and the startup check: `{"status":"ok","version":"…","schema_version":1}` |
| `GET` | `/api/tasks` | List tasks; query parameters narrow and order the result |
| `POST` | `/api/tasks` | Create a task; response `201` with the stored task and a `Location` header |
| `PATCH` | `/api/tasks/{id}` | Change the fields present in the body; response `200` with the stored task |
| `DELETE` | `/api/tasks/{id}` | Remove a task; response `204` |

`GET /api/tasks` returns `{"tasks":[…]}`. Query parameters mirror the CLI flags
exactly, because both are documented as the same feature:

| Parameter | Meaning | CLI equivalent |
| --- | --- | --- |
| `q` (repeatable) | Title substring, ANDed; a leading `~` excludes | `tada list TERM…` |
| `all=true` | Include completed tasks | `--all` |
| `overdue=true` | Uncompleted and past due | `--overdue` |
| `due_before=DATE` | Due before the date, ISO or a word like `tomorrow` | `--due-before` |
| `priority=0\|1\|2` | Exact priority | `--priority` |
| `sort=due\|priority\|created` | Ordering | `--sort` |

`POST` and `PATCH` accept `due_date` as an ISO date or any word the CLI accepts
(`tomorrow`, `fri`, `+3d`); the server normalises through
`model.ParseDueDate` before storing, so the file never depends on the client.
`PATCH` with `"due_date": ""` clears the date. An absent field is left alone,
which is exactly how `tada edit` behaves.

Validation is `model.NewTask` on the merged task, so the API cannot accept
anything the CLI would reject. Errors are JSON and carry a stable `code`:

```json
{"error":"priority 9 out of range","code":"invalid"}
```

| Status | `code` | When |
| --- | --- | --- |
| `400` | `bad_request` | Malformed JSON, unknown field, wrong content type |
| `404` | `not_found` | No task with that ID, or no such API route |
| `409` | `conflict` | The store reported a schema newer than this build |
| `503` | `lock_timeout` | Another process held the write lock past the timeout |
| `500` | `internal` | Anything unexpected; the message is safe to show, the cause is logged |

`503` is deliberate: a lock timeout is transient and the client may retry, which
is not true of `409`.

## Security model

The server is a local convenience, not a network service.

- **Loopback only.** The listener binds `127.0.0.1`, never `0.0.0.0`. There is no
  flag to expose it; if that changes it needs authentication first.
- **Host check.** Requests whose `Host` is not `127.0.0.1:<port>` or
  `localhost:<port>` are rejected with `403`, which closes DNS-rebinding holes
  that a loopback bind alone does not.
- **Origin check.** Mutating requests with an `Origin` header that is not the
  server's own origin, or with `Sec-Fetch-Site: cross-site`, are rejected. No
  CORS headers are ever sent, and `OPTIONS` is not answered.
- **JSON only for writes.** `POST` and `PATCH` require
  `Content-Type: application/json`, which a cross-origin HTML form cannot send
  without a preflight the server refuses.
- **No authentication, on purpose.** Any process running as you can already run
  `tada` or edit the file directly, so an API token would add ceremony without
  adding protection. What must not happen is a *web page* driving the API, which
  is what the Host, Origin and content-type checks above prevent.
- The threat model, including what this does *not* defend against (a compromised
  local account, or another user on the same machine if the file is
  world-writable), belongs in the README section added with the server.

## Front end

```
ui/assets/
  index.html        # shell: landmarks, <main>, the task list container
  favicon.svg       # embedded, no remote icon
  css/tokens.css    # colour, spacing, type scale, motion durations
  css/app.css       # layout and components
  js/main.js        # bootstrap and wiring
  js/api.js         # fetch wrappers, error mapping, one place for URLs
  js/state.js       # single store object, subscriptions, optimistic updates
  js/render.js      # DOM building for list, item, composer, editor
  js/keys.js        # keyboard map, single keydown handler
```

- **No framework, no virtual DOM.** One `state` object, one `render()` path, and
  keyed updates so that re-rendering a list does not destroy focus or restart
  animations. Replacing the whole list's `innerHTML` is explicitly not the
  approach; it loses the caret mid-edit.
- **Server-authoritative filtering.** Search, filters and sort call the API with
  query parameters instead of reimplementing matching in JavaScript. Loopback
  latency is under a millisecond, so a debounced keystroke round trip still feels
  immediate, and there is exactly one definition of what "overdue" means.
- **Optimistic writes with rollback.** Completing a task updates the row
  immediately, then reconciles with the response; a failure restores the row and
  surfaces the error. This is where the app feels fluid rather than merely fast.
- **Design language.** System font stack (no webfont), fluid type via `clamp()`,
  a neutral surface palette with one accent, hairline separators, 12px radii,
  and `prefers-color-scheme` for dark mode. Motion is 120–200ms ease-out, uses
  transform and opacity only, and disappears under `prefers-reduced-motion`.
  View Transitions are progressive enhancement, never load-bearing.
- **Accessibility baseline.** Semantic landmarks, a real list with correct roles,
  visible focus rings, `aria-live` for task creation and deletion, full keyboard
  operation, and 4.5:1 text contrast in both schemes.

## Testing

| Layer | How |
| --- | --- |
| API | `httptest` against a store on a temp file: CRUD, validation, 404s, filters and sort parity with the CLI, error codes |
| Middleware | Host, Origin and content-type rejections asserted with table tests |
| Embedding | Every asset is served, `index.html` is `text/html`, unknown non-API path falls back to the shell, API 404s stay JSON |
| Offline guarantee | Test greps the embedded assets for `http://`, `https://` and protocol-relative URLs, so a CDN import fails CI |
| Portability | CI builds the real binary, starts `tada ui --port 0 --no-open`, asserts `/` and `/api/tasks` answer, then stops it — on Linux and Windows |

The browser hand-off sits behind a function the command holds, so tests never
launch a browser and CI uses `--no-open`.

## Release slicing

**v0.5.0 — foundation.** The app is usable end to end: create, complete, edit
and delete tasks from a browser, with the same rules as the CLI.

1. Extract filters and sorting into `query/` (no behaviour change).
2. `tada ui` server skeleton: loopback bind, `--port`, `--no-open`, startup
   line, graceful shutdown, clear port-in-use failure.
3. Embedded asset pipeline and SPA fallback.
4. Read API with the query parameters above.
5. Write API: create, update, delete, with CLI-identical validation.
6. Localhost request hardening and the threat model in the README.
7. Page shell and design system.
8. Task list and CRUD interactions, including optimistic updates.

**v0.6.0 — fluidity and polish.** The app becomes pleasant rather than merely
functional.

9. Search, filter and sort controls driven by the API.
10. Keyboard-first operation and a shortcut overlay.
11. Motion system and view transitions.
12. Accessibility pass against the baseline above.
13. Performance and scale: a thousand tasks, no layout thrash, measured budget.
14. Cross-platform UI smoke test in CI and a release checklist update.

## Non-goals

- Remote or multi-user access. No auth, no sync, no server deployment.
- A native desktop shell (Electron, Wails, Tauri). They break the portability
  constraint, and the browser is already installed.
- Realtime collaboration, SSE or websockets while the CLI can edit the file
  behind the UI's back; refetch on focus covers the single-user case.
- Replacing the JSON store. SQLite would mean CGO or a second dependency and a
  migration the plan does not need.
- A second date vocabulary or a second set of filter rules.

## Risks

- **A second writer is already the design.** Two `tada ui` instances, or a UI
  plus a terminal, are expected; the lock is what makes it safe, and the API must
  never hold a lock across a response.
- **Windows asset paths.** Embedded FS uses forward slashes regardless of OS;
  serving through `http.FileServerFS` avoids hand-rolled path joins that break
  on Windows.
- **Bundle drift is impossible by construction**, which is the main reason for
  rejecting a framework; the cost is writing DOM code by hand, paid once.
