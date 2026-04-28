package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			if w.Code != tt.code {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.code, w.Body.String())
			}
			if !hasSchema(app.Schemas(), "bindQueryTestRequest") {
				t.Fatalf("BindQueryAndValidate did not register request schema")
			}
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
			if w.Code != tt.code {
				t.Fatalf("status = %d, want %d, body=%s", w.Code, tt.code, w.Body.String())
			}
			if !hasSchema(app.Schemas(), "bindPathTestRequest") {
				t.Fatalf("BindPathAndValidate did not register request schema")
			}
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
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			data := resp["data"].(map[string]any)
			if data["ip"] != tt.want {
				t.Fatalf("ip = %v, want %s", data["ip"], tt.want)
			}
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
	if w.Code != http.StatusOK {
		t.Fatalf("cookie status = %d, body=%s", w.Code, w.Body.String())
	}
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "seen=abc") || !strings.Contains(setCookie, "Path=/") || !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("unexpected Set-Cookie: %q", setCookie)
	}

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/redirect", nil))
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/target" {
		t.Fatalf("redirect status/location = %d/%q", w.Code, w.Header().Get("Location"))
	}

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad-redirect", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("bad redirect status = %d, want 500", w.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write static file: %v", err)
	}

	app := New()
	app.Static("/static", dir)

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/hello.txt", nil))
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "hello" {
		t.Fatalf("static hit status/body = %d/%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/missing.txt", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("static miss status = %d, want 404", w.Code)
	}
}
