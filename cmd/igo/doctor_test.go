package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func parseSource(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return fset, f
}

func TestCheckErrShouldWrap_Triggers(t *testing.T) {
	src := `package x
func h(c *Context) {
	_, err := doStuff()
	if err != nil {
		c.InternalError("failed")
	}
}
func doStuff() (int, error) { return 0, nil }`
	fset, f := parseSource(t, src)
	diags := checkErrShouldWrap(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "should-wrap" {
		t.Errorf("rule = %s, want should-wrap", diags[0].Rule)
	}
	if !strings.Contains(diags[0].Message, "InternalErrorWrap") {
		t.Errorf("message missing replacement hint: %s", diags[0].Message)
	}
}

func TestCheckErrShouldWrap_IgnoresRecoverPattern(t *testing.T) {
	// `if err := recover(); err != nil` 中 err 是 interface{}，不该警告
	src := `package x
func setup() {
	defer func() {
		if err := recover(); err != nil {
			doSomething()
			panic(err)
		}
	}()
}
func doSomething() {}`
	fset, f := parseSource(t, src)
	diags := checkErrShouldWrap(fset, f)
	if len(diags) != 0 {
		t.Errorf("recover() pattern should not trigger should-wrap, got %d: %+v", len(diags), diags)
	}
}

func TestCheckErrShouldWrap_IgnoresOutsideIfBlock(t *testing.T) {
	src := `package x
func h(c *Context) {
	c.InternalError("standalone")
}`
	fset, f := parseSource(t, src)
	diags := checkErrShouldWrap(fset, f)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckGroupInternalUse_Triggers(t *testing.T) {
	src := `package x
func setup(app *App) {
	app.Group("/api", func(v1 *App) {
		v1.Use(authMiddleware)
		v1.Get("/users", listUsers)
	})
}`
	fset, f := parseSource(t, src)
	diags := checkGroupInternalUse(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "group-use-leak" {
		t.Errorf("rule = %s, want group-use-leak", diags[0].Rule)
	}
}

func TestCheckGroupInternalUse_OK(t *testing.T) {
	src := `package x
func setup(app *App) {
	app.Group("/api", func(v1 *App) {
		v1.Get("/users", listUsers)
	}, authMiddleware)
}`
	fset, f := parseSource(t, src)
	diags := checkGroupInternalUse(fset, f)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckAppUseMissingNext_Triggers(t *testing.T) {
	src := `package x
func setup(app *App) {
	app.Use(func(c *Context) {
		c.Header("X-Foo", "bar")
	})
}`
	fset, f := parseSource(t, src)
	diags := checkAppUseMissingNext(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "middleware-missing-next" {
		t.Errorf("rule = %s, want middleware-missing-next", diags[0].Rule)
	}
}

func TestCheckAppUseMissingNext_HasNext(t *testing.T) {
	src := `package x
func setup(app *App) {
	app.Use(func(c *Context) {
		c.Header("X-Foo", "bar")
		c.Next()
	})
}`
	fset, f := parseSource(t, src)
	diags := checkAppUseMissingNext(fset, f)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckAppUseMissingNext_ShortCircuits(t *testing.T) {
	src := `package x
func setup(app *App) {
	app.Use(func(c *Context) {
		if c.Request.Header.Get("X-Auth") == "" {
			c.Unauthorized("missing token")
			return
		}
		c.Next()
	})
}`
	fset, f := parseSource(t, src)
	diags := checkAppUseMissingNext(fset, f)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckMissingReturnAfterErrorResponse_Triggers(t *testing.T) {
	src := `package x
func h(c *Context) {
	if invalid() {
		c.BadRequest("invalid")
		doWork()
	}
}
func invalid() bool { return true }
func doWork() {}`
	fset, f := parseSource(t, src)
	diags := checkMissingReturnAfterErrorResponse(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "missing-return-after-error" {
		t.Errorf("rule = %s, want missing-return-after-error", diags[0].Rule)
	}
}

func TestCheckMissingReturnAfterErrorResponse_OK(t *testing.T) {
	src := `package x
func h(c *Context) {
	if invalid() {
		c.BadRequest("invalid")
		return
	}
}
func invalid() bool { return true }`
	fset, f := parseSource(t, src)
	diags := checkMissingReturnAfterErrorResponse(fset, f)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(diags))
	}
}

func TestCheckDoubleSuccessResponse_Triggers(t *testing.T) {
	src := `package x
func h(c *Context) {
	c.Success("first")
	c.Created("second")
}`
	fset, f := parseSource(t, src)
	diags := checkDoubleSuccessResponse(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "multiple-success-responses" {
		t.Errorf("rule = %s, want multiple-success-responses", diags[0].Rule)
	}
}

func TestCheckJSONErrorResponse_Triggers(t *testing.T) {
	src := `package x
func h(c *Context) {
	_, err := doStuff()
	if err != nil {
		c.JSON(500, err)
		return
	}
}
func doStuff() (int, error) { return 0, nil }`
	fset, f := parseSource(t, src)
	diags := checkJSONErrorResponse(fset, f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Rule != "json-error" {
		t.Errorf("rule = %s, want json-error", diags[0].Rule)
	}
}
