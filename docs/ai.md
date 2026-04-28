# igo AI Integration

This document is the stable entry point for AI coding tools using igo.

## Recommended Flow

1. `go run ./cmd/igo ai context . --format json`
2. `go run ./cmd/igo ai routes .`
3. `go run ./cmd/igo ai schemas .`
4. `go run ./cmd/igo ai prompt . METHOD PATH` when editing one handler
5. `go test ./...`
6. `go run ./cmd/igo doctor .`

For a running app, call `app.RegisterAIRoutes()` and inspect:

- `/_ai/routes`
- `/_ai/schemas`
- `/_ai/errors`
- `/_ai/info`
- `/_ai/openapi`
- `/_ai/conventions`
- `/_ai/middlewares`

## Metadata Contract

Routes use `core/route.RouteConfig`:

- `method`, `path`, `summary`, `description`
- `handlerName`, `filePath`, `lineNumber`
- `params`, `requestBody`, `responses`
- `tags`, `aiHints`, `middlewares`

Schemas use `types.TypeSchema`:

- `name`, `package`, `filePath`
- fields with `goName`, `jsonName`, `goType`, `type`
- validation fields: `validate`, `required`, `enum`, `min`, `max`, `gte`, `lte`, `len`

The same models feed `igo ai routes`, `igo ai schemas`, `igo ai openapi`, `/_ai/routes`, `/_ai/schemas`, and `/_ai/openapi`.

## Handler Pattern

```go
func listUsers(c *igo.Context) {
	q, ok := igo.BindQueryAndValidate[ListUsersQuery](c)
	if !ok {
		return
	}

	users, err := svc.List(c.Request.Context(), q)
	if err != nil {
		c.InternalErrorWrap(err, "failed to list users", nil)
		return
	}

	c.Success(users)
}
```

## Response DTO Registration

Request DTOs are registered automatically when bound. Response-only DTOs are not visible unless registered:

```go
app.RegisterSchema(UserResponse{})
igo.RegisterAppSchema[UserResponse](app)
```

`igo.RegisterSchema[T]()` still exists for legacy global registration, but new code should prefer App-owned registration to avoid multi-App pollution.

## OpenAPI

OpenAPI generation includes:

- `paths`
- path/query parameters
- request bodies
- responses
- `components.schemas`

Unknown or dynamic Go values such as `core.H`, map literals, and complex expressions are represented as `type: object` instead of dangling `$ref` values.
