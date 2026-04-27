package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/igo/igo/core"
)

func TestLogger(t *testing.T) {
	app := core.New()

	app.Use(Logger())
	app.Get("/test", func(c *core.Context) {
		c.Success(core.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)
}

func TestRecovery(t *testing.T) {
	app := core.New()

	app.Use(Recovery())
	app.Get("/panic", func(c *core.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestCORS(t *testing.T) {
	app := core.New()

	app.Use(CORS())
	app.Get("/cors", func(c *core.Context) {
		c.Success(core.H{"ok": true})
	})

	// Test OPTIONS preflight
	req := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for OPTIONS, got %d", w.Code)
	}

	// Test regular request
	req = httptest.NewRequest(http.MethodGet, "/cors", nil)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAuth(t *testing.T) {
	app := core.New()

	app.Use(Auth())
	app.Get("/secure", func(c *core.Context) {
		c.Success(core.H{"secret": "data"})
	})

	// Without token
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 without auth, got %d", w.Code)
	}

	// With token
	req = httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with auth, got %d", w.Code)
	}
}

func TestRequestID(t *testing.T) {
	app := core.New()

	app.Use(RequestID())
	app.Get("/id", func(c *core.Context) {
		c.Success(core.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header to be set")
	}

	// Test with existing ID
	req = httptest.NewRequest(http.MethodGet, "/id", nil)
	req.Header.Set("X-Request-ID", "custom-id")
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "custom-id" {
		t.Errorf("expected X-Request-ID 'custom-id', got '%s'", w.Header().Get("X-Request-ID"))
	}
}

func TestRateLimit(t *testing.T) {
	app := core.New()

	app.Use(RateLimit(100, 0))
	app.Get("/limited", func(c *core.Context) {
		c.Success(core.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
