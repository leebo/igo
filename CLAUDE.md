# igo Framework - AI-Friendly Go Web Framework

igo is a small Go web framework shaped for AI coding tools: low-boilerplate handlers, structured validation, stable error responses, compact CLI context, and runtime `/_ai/*` metadata.

## TL;DR

```go
func createUser(c *igo.Context) {
	req, ok := igo.BindAndValidate[CreateUserRequest](c)
	if !ok {
		return
	}

	user, err := svc.Create(c.Request.Context(), req)
	if err != nil {
		c.InternalErrorWrap(err, "failed to create user", map[string]any{"email": req.Email})
		return
	}

	c.Created(user)
}
```

```go
func main() {
	app := igo.New()
	app.Get("/health", health)
	app.Post("/users", createUser)
	app.RegisterSchema(UserResponse{})
	app.RegisterAIRoutes()
	app.Run(":8080")
}
```

## Handler Rules

- Bind input, call service/repository code, send exactly one response.
- JSON body: `igo.BindAndValidate[T](c)`.
- Query params: `igo.BindQueryAndValidate[T](c)` or `c.Query*` helpers.
- Path params: `igo.BindPathAndValidate[T](c)` or `c.Param*OrFail` helpers.
- After any helper returns `ok == false`, return immediately.
- In error branches, prefer wrap helpers: `BadRequestWrap`, `NotFoundWrap`, `ValidationErrorWrap`, `InternalErrorWrap`.
- Register response-only DTOs with `app.RegisterSchema(ResponseDTO{})` or `igo.RegisterAppSchema[ResponseDTO](app)`.

## Routing

```go
app.Get("/path", handler)
app.Get("/path", handler, middleware.Auth())
app.Group("/api/v1", func(v1 *igo.App) {
	v1.Get("/users", listUsers)
	v1.Post("/users", createUser)
}, middleware.Auth())
app.Resources("/users", igo.ResourceHandler{List: listUsers, Show: getUser})
app.Static("/static", "./public")
```

Group middleware belongs in the `Group` argument list. Do not call `Use` inside the group closure unless the intent is global mutation.

## Responses

- Success: `c.Success(data)`, `c.Created(data)`, `c.NoContent()`, `c.JSON(status, value)`.
- Plain errors: `c.BadRequest(msg)`, `c.Unauthorized(msg)`, `c.Forbidden(msg)`, `c.NotFound(msg)`, `c.InternalError(msg)`.
- Wrapped errors: `c.BadRequestWrap(err, msg)`, `c.NotFoundWrap(err, msg)`, `c.ValidationErrorWrap(err, field, msg)`, `c.InternalErrorWrap(err, msg, meta)`.
- Sentinels: `c.FailIfError(err, msg)`, `c.NotFoundIfNotFound(err, "user")`, `c.SuccessIfNotNil(value, "user")`.

## Error Codes

| Code | Status | Preferred helper |
|------|--------|------------------|
| `BAD_REQUEST` | 400 | `c.BadRequestWrap(err, msg)` |
| `UNAUTHORIZED` | 401 | `c.Unauthorized(msg)` |
| `FORBIDDEN` | 403 | `c.Forbidden(msg)` |
| `NOT_FOUND` | 404 | `c.NotFoundWrap(err, msg)` |
| `VALIDATION_FAILED` | 422 | `c.ValidationError(err)` |
| `INTERNAL_ERROR` | 500 | `c.InternalErrorWrap(err, msg, meta)` |
| `INVALID_JSON` | 400 | internal, surfaced via `BadRequestWrap` |

The canonical machine-readable list is `go run ./cmd/igo ai errors` and `GET /_ai/errors`.

## Schema Model

`types.TypeSchema` is shared by CLI and runtime endpoints. It includes:

- `name`, `package`, `filePath`
- field `goName`, `jsonName`, `goType`, JSON `type`
- `validate`, `required`, `enum`, `min`, `max`, `gte`, `lte`, `len`, `description`

Request schemas are auto-registered when handlers call `BindAndValidate`, `BindQueryAndValidate`, or `BindPathAndValidate`. Response-only DTOs must be registered explicitly.

## AI CLI

```bash
go run ./cmd/igo ai context . --format md
go run ./cmd/igo ai context . --format json
go run ./cmd/igo ai routes .
go run ./cmd/igo ai schemas .
go run ./cmd/igo ai errors
go run ./cmd/igo ai openapi .
go run ./cmd/igo ai prompt . GET /users/:id
go run ./cmd/igo ai workflow
go run ./cmd/igo ai examples
```

`routes`, `schemas`, `errors`, and `openapi` consume the same metadata model. OpenAPI output includes `components.schemas` and avoids dangling `$ref` values.

## Runtime AI Endpoints

Enable with:

```go
app.RegisterAIRoutes()             // dev/test only; no-op in prd
app.RegisterAIRoutesUnsafe()       // explicit opt-in for prd
```

`igo.Simple()` already calls `RegisterAIRoutes()` automatically when mode != prd, so most apps don't need to call it manually.

Endpoints:

- `GET /_ai/routes`
- `GET /_ai/schemas`
- `GET /_ai/errors`
- `GET /_ai/info` — includes `mode` ("dev"/"test"/"prd"), and in dev `dev_endpoint` + `dev_events` pointing at the watcher's `/_ai/dev` (when started via `igo dev`)
- `GET /_ai/openapi`
- `GET /_ai/conventions`
- `GET /_ai/middlewares`

`/_ai/middlewares` returns global middleware names, route middleware names, and registration order. Function names are inferred with `runtime.FuncForPC`.

## Environment Modes

igo apps run in one of three modes, selected by the `IGO_ENV` environment variable:

| Mode  | When                              | Logger     | CORS default | Recovery       | `/_ai/*` auto |
|-------|-----------------------------------|------------|--------------|----------------|---------------|
| `dev` | unset / `dev` / `development`     | verbose    | `*`          | stack visible  | yes           |
| `test`| `test` / `testing`                | silent     | `*`          | stack visible  | yes           |
| `prd` | `prd` / `prod` / `production`     | structured | deny-all + WARN log | stack hidden | **no** (must call `RegisterAIRoutesUnsafe()`) |

Use `core.DetectMode()`, `core.ModeDev/ModeTest/ModePrd`, or `app.Mode.IsPrd()` predicates. `app.WithMode(m)` is available for tests.

When using `igo.Simple()`, middleware defaults switch automatically. When using `igo.New()`, you can compose explicitly:

```go
app := igo.New()
app.Use(middleware.RecoveryFor(app.Mode))
app.Use(middleware.CORSFor(app.Mode))
app.Use(middleware.LoggerFor(app.Mode))
```

## Hot Reload (dev mode)

`igo dev` is a watcher process that rebuilds + restarts your app on file save and exposes structured state for AI clients.

```bash
cd examples/dev_demo
go run ../../cmd/igo dev                                  # default: watch cwd
go run ../../cmd/igo dev --dir . --watcher-port 18999 --app-addr :8080
```

While running:

- `GET http://127.0.0.1:18999/_ai/dev` — full state JSON: mode, build phase, last reload, compile errors, watched roots, ports
- `GET http://127.0.0.1:18999/_ai/dev/events` — Server-Sent Events stream pushing `build:start`, `build:ok`, `build:fail`, `reload:done`. Subscribe once and wait, instead of polling.
- The child app's `/_ai/info` exposes `dev_endpoint` and `dev_events` URLs so the AI client can discover the watcher

**For AI clients**: prefer one `GET /_ai/dev` (or one `curl -N /_ai/dev/events`) over running `go build` repeatedly. The watcher already ran the build; the result is sitting in `compile_errors` (with `type`, `file:line`, `message`, `suggestion`).

Compile errors are auto-classified into: `undefined_symbol`, `missing_import`, `type_mismatch`, `syntax`, `unknown`. Each carries a one-line suggestion.

## Releases

`igo release` automates SemVer tagging + CHANGELOG generation from
[Conventional Commits](https://www.conventionalcommits.org/).

```bash
go run ./cmd/igo release --dry-run    # preview the next version + entry
go run ./cmd/igo release              # write CHANGELOG, commit, tag (no push)
go run ./cmd/igo release --push       # also push commit + tag to origin
go run ./cmd/igo release --bump minor # override auto-detection
```

Bump policy (while in v0.x):

| Commit kind | Bump |
|-------------|------|
| Has `feat!:` / `fix!:` / `BREAKING CHANGE:` | **MINOR** (0.x rule, not MAJOR) |
| Has `feat:` | PATCH |
| Has `fix:` / `perf:` | PATCH |
| Only `chore:` / `test:` / `ci:` / `docs:` / `style:` | refuses, requires `--bump` to force |

Once the project ships v1.0.0 the tool switches to standard SemVer
(BREAKING → MAJOR, feat → MINOR, fix → PATCH) automatically.

The release commit format is `chore(release): vX.Y.Z`. Tag is annotated
with the same content as the new CHANGELOG entry.

## AI Workflow

1. Run `go run ./cmd/igo ai context . --format json`.
2. Inspect `go run ./cmd/igo ai routes .` and `go run ./cmd/igo ai schemas .`.
3. Use `go run ./cmd/igo ai prompt . METHOD PATH` when editing one route.
4. Validate with `go test ./...` and `go run ./cmd/igo doctor .`.
5. If the app is running, compare CLI output with `/_ai/routes`, `/_ai/schemas`, `/_ai/errors`, and `/_ai/openapi`.

## Validation Rules

Supported tags include `required`, `email`, `min`, `max`, `gte`, `lte`, `gt`, `lt`, `len`, `regex`, `uuid`, `url`, `enum`, and `eqfield`.

```go
type ListQuery struct {
	Page  int    `json:"page" validate:"gte:1"`
	Size  int    `json:"size" validate:"lte:100"`
	Email string `json:"email" validate:"omitempty|email"`
}
```

## Verification

Before handoff:

```bash
go test ./...
go run ./cmd/igo doctor .
go run ./cmd/igo ai context ./examples/full --format json
go run ./cmd/igo ai routes ./examples/full
go run ./cmd/igo ai schemas ./examples/full
go run ./cmd/igo ai errors
go run ./cmd/igo ai openapi ./examples/full
```
