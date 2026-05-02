# igo Agent Guide

Short context for Codex, Claude Code, Cursor, and other AI coding agents.

## Core Rules

- Keep handlers thin: bind request data, call a service, send one response.
- Use `core.BindAndValidate[T](c)` for JSON bodies. If `ok` is false, return immediately.
- Use `core.BindQueryAndValidate[T](c)` for structured query parameters.
- Use `core.BindPathAndValidate[T](c)` or `c.Param*OrFail` helpers for path parameters.
- In `if err != nil` branches, prefer `c.InternalErrorWrap`, `c.NotFoundWrap`, `c.BadRequestWrap`, or `c.ValidationErrorWrap` so the original error is preserved.
- Put group middleware in `app.Group("/prefix", fn, middleware)` arguments. Do not call `group.Use(...)` inside the group closure.
- Register pure response DTOs with `app.RegisterSchema(ResponseDTO{})` or `igo.RegisterAppSchema[ResponseDTO](app)`.
- Run `go test ./...` and `go run ./cmd/igo doctor .` before handing off broad changes.

## AI Workflow

1. Run `go run ./cmd/igo ai context . --format json` for compact project facts.
2. Inspect `go run ./cmd/igo ai routes .` and `go run ./cmd/igo ai schemas .` before changing handlers or DTOs.
3. Use `go run ./cmd/igo ai prompt . METHOD PATH` for route-specific context.
4. Validate with `go test ./...` and `go run ./cmd/igo doctor .`.
5. For a running app, compare CLI output with `/_ai/routes`, `/_ai/schemas`, `/_ai/errors`, and `/_ai/openapi`.

## AI Commands

- `go run ./cmd/igo ai context . --format md|json`
- `go run ./cmd/igo ai routes .`
- `go run ./cmd/igo ai schemas .`
- `go run ./cmd/igo ai errors`
- `go run ./cmd/igo ai openapi .`
- `go run ./cmd/igo ai prompt . METHOD PATH`
- `go run ./cmd/igo ai workflow`
- `go run ./cmd/igo ai examples`

## Runtime Endpoints

`igo.Simple()` auto-calls `app.RegisterAIRoutes()` in dev/test mode. In prd, you must opt in with `app.RegisterAIRoutesUnsafe()`.

- `GET /_ai/routes`
- `GET /_ai/schemas`
- `GET /_ai/errors`
- `GET /_ai/info` — includes `mode` and (in dev) `dev_endpoint` / `dev_events`
- `GET /_ai/openapi`
- `GET /_ai/conventions`
- `GET /_ai/middlewares`

## Environment Modes

`IGO_ENV` selects mode at startup:

- unset / `dev` / `development` → `dev` (default, AI endpoints on)
- `test` / `testing` → `test`
- `prd` / `prod` / `production` → `prd` (AI endpoints off, strict CORS, no stack in errors)

Predicates: `app.Mode.IsDev()`, `IsTest()`, `IsPrd()`. Tests can override with `app.WithMode(core.ModeTest)`.

Mode-aware middleware factories (use these instead of plain `Logger()/Recovery()/CORS()` when composing manually):

- `middleware.LoggerFor(mode)` — verbose in dev, silent in test, structured in prd
- `middleware.RecoveryFor(mode)` — stack in dev/test response, only in log in prd
- `middleware.CORSFor(mode)` — `*` in dev/test, deny-all + WARN in prd

## Hot Reload (`igo dev`)

```bash
go run ./cmd/igo dev --dir ./examples/dev_demo
```

The watcher rebuilds + restarts your app on `*.go` save (excludes `_test.go`, `vendor/`, `.git/`, `node_modules/`, dotdirs). Default watcher port `:18999`.

**For AI clients: prefer the watcher endpoints over running `go build` yourself.**

- `GET /_ai/dev` (watcher port) — one-shot full state: mode, build phase, last reload, compile errors with `{file, line, type, message, suggestion}`, watched roots
- `GET /_ai/dev/events` — SSE stream: `build:start`, `build:ok`, `build:fail`, `reload:done`

Subscribe once and wait — never poll `go build` in a loop.

## Releases (`igo release`)

Auto-tags + writes CHANGELOG.md from Conventional Commits.

```bash
go run ./cmd/igo release --dry-run     # preview next version
go run ./cmd/igo release               # cut local release (no push)
go run ./cmd/igo release --push        # release and push
go run ./cmd/igo release --bump minor  # force a level
```

While in v0.x: BREAKING → MINOR, feat/fix → PATCH. After v1.0.0: standard
SemVer. Refuses to release on dirty tree, on duplicate tag, or when only
chore/test/ci/docs/style commits exist (use `--bump` to force).

## Common Pattern

```go
func (h *UserHandler) Create(c *core.Context) {
	req, ok := core.BindAndValidate[models.CreateUserRequest](c)
	if !ok {
		return
	}

	created, err := h.service.CreateUser(c.Request.Context(), req)
	if err != nil {
		c.InternalErrorWrap(err, "failed to create user", map[string]any{
			"email": req.Email,
		})
		return
	}

	c.Created(created)
}
```
