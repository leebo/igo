package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leebo/igo/core"
	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, http.StatusInternalServerError, w.Code)
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

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Test regular request
	req = httptest.NewRequest(http.MethodGet, "/cors", nil)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
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

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With token
	req = httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))

	// Test with existing ID
	req = httptest.NewRequest(http.MethodGet, "/id", nil)
	req.Header.Set("X-Request-ID", "custom-id")
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, "custom-id", w.Header().Get("X-Request-ID"))
}
