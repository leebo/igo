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
app.RegisterAIRoutes()
```

Endpoints:

- `GET /_ai/routes`
- `GET /_ai/schemas`
- `GET /_ai/errors`
- `GET /_ai/info`
- `GET /_ai/openapi`
- `GET /_ai/conventions`
- `GET /_ai/middlewares`

`/_ai/middlewares` returns global middleware names, route middleware names, and registration order. Function names are inferred with `runtime.FuncForPC`.

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
