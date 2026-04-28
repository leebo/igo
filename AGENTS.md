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

Call `app.RegisterAIRoutes()` once during setup:

- `GET /_ai/routes`
- `GET /_ai/schemas`
- `GET /_ai/errors`
- `GET /_ai/info`
- `GET /_ai/openapi`
- `GET /_ai/conventions`
- `GET /_ai/middlewares`

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
