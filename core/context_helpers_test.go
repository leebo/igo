package core

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bindQueryTestRequest struct {
	Page int    `json:"page" validate:"gte:1|lte:10"`
	Name string `json:"name" validate:"max:20"`
}

type bindPathTestRequest struct {
	ID int64 `json:"id" validate:"gte:1|lte:10"`
}

func TestBindQueryAndValidate(t *testing.T) {
	tests := []struct {
		name string
		url  string
		code int
	}{
		{name: "success", url: "/items?page=2&name=ada", code: http.StatusOK},
		{name: "bind failure", url: "/items?page=abc", code: http.StatusBadRequest},
		{name: "validation failure", url: "/items?page=11", code: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			app.Get("/items", func(c *Context) {
				req, ok := BindQueryAndValidate[bindQueryTestRequest](c)
				if !ok {
					return
				}
				c.Success(H{"page": req.Page, "name": req.Name})
			})

			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.url, nil))
			require.Equal(t, tt.code, w.Code, w.Body.String())
			require.True(t, hasSchema(app.Schemas(), "bindQueryTestRequest"))
		})
	}
}

func TestBindPathAndValidate(t *testing.T) {
	tests := []struct {
		name string
		url  string
		code int
	}{
		{name: "success", url: "/items/7", code: http.StatusOK},
		{name: "bind failure", url: "/items/abc", code: http.StatusBadRequest},
		{name: "validation failure", url: "/items/11", code: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New()
			app.Get("/items/:id", func(c *Context) {
				req, ok := BindPathAndValidate[bindPathTestRequest](c)
				if !ok {
					return
				}
				c.Success(H{"id": req.ID})
			})

			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.url, nil))
			require.Equal(t, tt.code, w.Code, w.Body.String())
			require.True(t, hasSchema(app.Schemas(), "bindPathTestRequest"))
		})
	}
}

func TestClientIPPriority(t *testing.T) {
	app := New()
	app.Get("/ip", func(c *Context) {
		c.Success(H{"ip": c.ClientIP()})
	})

	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name: "x forwarded for",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10, 10.0.0.1",
				"X-Real-IP":       "203.0.113.11",
			},
			remoteAddr: "192.0.2.10:1234",
			want:       "203.0.113.10",
		},
		{
			name: "x real ip",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.11",
			},
			remoteAddr: "192.0.2.10:1234",
			want:       "203.0.113.11",
		},
		{
			name:       "remote addr",
			remoteAddr: "192.0.2.10:1234",
			want:       "192.0.2.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			var resp H
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			data := resp["data"].(map[string]any)
			require.Equal(t, tt.want, data["ip"])
		})
	}
}

func TestCookieSetCookieAndRedirect(t *testing.T) {
	app := New()
	app.Get("/cookie", func(c *Context) {
		value, err := c.Cookie("session")
		if err != nil {
			c.BadRequest("missing cookie")
			return
		}
		c.SetCookie("seen", value, 60, "", "", false, true)
		c.Success(H{"session": value})
	})
	app.Get("/redirect", func(c *Context) {
		c.Redirect(http.StatusFound, "/target")
	})
	app.Get("/bad-redirect", func(c *Context) {
		c.Redirect(http.StatusOK, "/target")
	})

	req := httptest.NewRequest(http.MethodGet, "/cookie", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	setCookie := w.Header().Get("Set-Cookie")
	assert.Contains(t, setCookie, "seen=abc")
	assert.Contains(t, setCookie, "Path=/")
	assert.Contains(t, setCookie, "HttpOnly")

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/redirect", nil))
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/target", w.Header().Get("Location"))

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad-redirect", nil))
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStaticFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644))

	app := New()
	app.Static("/static", dir)

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/hello.txt", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", strings.TrimSpace(w.Body.String()))

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/missing.txt", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestParamAndQueryFailHelpers(t *testing.T) {
	app := New()
	app.Get("/items/:id/:active", func(c *Context) {
		id, ok := c.ParamInt64OrFail("id")
		if !ok {
			return
		}
		page, ok := c.QueryInt64OrFail("page")
		if !ok {
			return
		}
		c.Success(H{
			"id":      id,
			"page":    page,
			"active":  c.ParamBool("active"),
			"verbose": c.QueryBool("verbose", false),
			"missing": c.QueryInt64("missing", 9),
		})
	})

	tests := []struct {
		path string
		code int
	}{
		{"/items/7/yes?page=2&verbose=true", http.StatusOK},
		{"/items/bad/yes?page=2", http.StatusBadRequest},
		{"/items/7/yes", http.StatusBadRequest},
		{"/items/7/yes?page=bad", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			assert.Equal(t, tt.code, w.Code)
		})
	}
}

func TestContextResultHelpers(t *testing.T) {
	app := New()
	app.Get("/success", func(c *Context) {
		value := H{"ok": true}
		assert.True(t, c.SuccessIfNotNil(value, "item"))
	})
	app.Get("/success-nil", func(c *Context) {
		var value *struct{}
		assert.False(t, c.SuccessIfNotNil(value, "item"))
	})
	app.Get("/not-found-err", func(c *Context) {
		assert.True(t, c.NotFoundIfNotFound(stderrors.New("missing"), "item"))
	})
	app.Get("/success-or-fail", func(c *Context) {
		assert.True(t, c.SuccessIfNotNilOrFail(H{"ok": true}, nil, "item"))
	})
	app.Get("/success-or-fail-err", func(c *Context) {
		assert.True(t, c.SuccessIfNotNilOrFail(nil, stderrors.New("missing"), "item"))
	})
	app.Get("/fail", func(c *Context) {
		assert.True(t, c.FailIfError(stderrors.New("boom"), "failed"))
	})
	app.Get("/fail-meta", func(c *Context) {
		assert.True(t, c.FailIfErrorWithMeta(stderrors.New("boom"), "failed", map[string]any{"key": "value"}))
	})

	tests := map[string]int{
		"/success":             http.StatusOK,
		"/success-nil":         http.StatusNotFound,
		"/not-found-err":       http.StatusNotFound,
		"/success-or-fail":     http.StatusOK,
		"/success-or-fail-err": http.StatusNotFound,
		"/fail":                http.StatusInternalServerError,
		"/fail-meta":           http.StatusInternalServerError,
	}

	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, want, w.Code, w.Body.String())
		})
	}
}

func TestContextHandlerControlHelpers(t *testing.T) {
	c := newContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), nil)

	c.SetHandlers([]HandlerFunc{
		func(c *Context) { c.Next() },
		func(c *Context) { c.Abort() },
	})
	assert.Equal(t, -1, c.GetHandlerIndex())
	c.SetHandlerIndex(0)
	assert.Equal(t, 0, c.GetHandlerIndex())
	c.Abort()
	assert.True(t, c.IsAborted())
	assert.Equal(t, 0, c.StatusCode())
}
