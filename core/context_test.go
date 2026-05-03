package core

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_Query(t *testing.T) {
	app := New()

	app.Get("/search", func(c *Context) {
		q := c.Query("q")
		c.Success(H{"query": q})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=hello", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "hello", data["query"])
}

func TestContext_QueryInt(t *testing.T) {
	app := New()

	app.Get("/page", func(c *Context) {
		page := c.QueryInt("page", 1)
		size := c.QueryInt("size", 20)
		c.Success(H{"page": page, "size": size})
	})

	tests := []struct {
		url      string
		wantPage int
		wantSize int
	}{
		{"/page", 1, 20},
		{"/page?page=2", 2, 20},
		{"/page?page=3&size=50", 3, 50},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			var resp H
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			data := resp["data"].(map[string]interface{})
			assert.Equal(t, tt.wantPage, int(data["page"].(float64)))
			assert.Equal(t, tt.wantSize, int(data["size"].(float64)))
		})
	}
}

func TestContext_QueryDefault(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Context) {
		val := c.QueryDefault("name", "default")
		c.Success(H{"name": val})
	})

	// Test with default
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "default", data["name"])

	// Test with value
	req = httptest.NewRequest(http.MethodGet, "/test?name=custom", nil)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data = resp["data"].(map[string]interface{})
	assert.Equal(t, "custom", data["name"])
}

func TestContext_Param(t *testing.T) {
	app := New()

	app.Get("/users/:id/posts/:postId", func(c *Context) {
		id := c.Param("id")
		postId := c.Param("postId")
		c.Success(H{"userId": id, "postId": postId})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "123", data["userId"])
	assert.Equal(t, "456", data["postId"])
}

func TestContext_BindJSON(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		var data struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		if err := c.BindJSON(&data); err != nil {
			c.BadRequest("invalid json")
			return
		}
		c.Success(H{"name": data.Name, "value": data.Value})
	})

	body := `{"name":"test","value":42}`
	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "test", data["name"])
	assert.Equal(t, 42, int(data["value"].(float64)))
}

func TestContext_BindJSON_Invalid(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.BindJSON(&data); err != nil {
			c.BadRequest("invalid json")
			return
		}
		c.Success(H{"name": data.Name})
	})

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestContext_BindQuery(t *testing.T) {
	app := New()

	app.Get("/search", func(c *Context) {
		var params struct {
			Query string `json:"q"`
			Limit int    `json:"limit"`
		}
		if err := c.BindQuery(&params); err != nil {
			c.BadRequest("invalid query")
			return
		}
		c.Success(H{"q": params.Query, "limit": params.Limit})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test&limit=10", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "test", data["q"])
	assert.Equal(t, 10, int(data["limit"].(float64)))
}

func TestContext_Success(t *testing.T) {
	app := New()

	app.Get("/ok", func(c *Context) {
		c.Success(H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotNil(t, resp["data"])
}

func TestContext_Created(t *testing.T) {
	app := New()

	app.Post("/resource", func(c *Context) {
		c.Created(H{"id": 1})
	})

	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestContext_NoContent(t *testing.T) {
	app := New()

	app.Delete("/resource", func(c *Context) {
		c.NoContent()
	})

	req := httptest.NewRequest(http.MethodDelete, "/resource", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestContext_BadRequest(t *testing.T) {
	app := New()

	app.Post("/data", func(c *Context) {
		c.BadRequest("invalid input")
	})

	req := httptest.NewRequest(http.MethodPost, "/data", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "BAD_REQUEST", errObj["code"])
}

func TestContext_NotFound(t *testing.T) {
	app := New()

	app.Get("/missing", func(c *Context) {
		c.NotFound("resource not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestContext_Unauthorized(t *testing.T) {
	app := New()

	app.Get("/secure", func(c *Context) {
		c.Unauthorized("token required")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestContext_Forbidden(t *testing.T) {
	app := New()

	app.Get("/forbidden", func(c *Context) {
		c.Forbidden("access denied")
	})

	req := httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestContext_InternalError(t *testing.T) {
	app := New()

	app.Get("/error", func(c *Context) {
		c.InternalError("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestContext_ValidationError(t *testing.T) {
	app := New()

	app.Post("/validate", func(c *Context) {
		c.ValidationError(ErrBodyRequired)
	})

	req := httptest.NewRequest(http.MethodPost, "/validate", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestContext_Header(t *testing.T) {
	app := New()

	app.Get("/headers", func(c *Context) {
		c.Header("X-Custom", "value")
		c.Header("X-Another", "another")
		c.Success(H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, "value", w.Header().Get("X-Custom"))
	assert.Equal(t, "another", w.Header().Get("X-Another"))
}

func TestContext_Status(t *testing.T) {
	app := New()

	app.Get("/custom", func(c *Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestContext_JSON(t *testing.T) {
	app := New()

	app.Get("/json", func(c *Context) {
		c.JSON(http.StatusOK, H{"json": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

type schemaUsageRequest struct {
	Name string `json:"name" validate:"required"`
}

type schemaUsageResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type schemaUsageQuery struct {
	Page int `json:"page"`
}

type schemaUsagePath struct {
	ID int64 `json:"id"`
}

func TestErrorResponse_TraceIDInjected(t *testing.T) {
	app := New()

	// 模拟 RequestID middleware
	app.Use(func(c *Context) {
		c.Set(CtxKeyRequestID, "trace-abc")
		c.Header("X-Request-ID", "trace-abc")
		c.Next()
	})

	cases := []struct {
		name    string
		handler func(c *Context)
		status  int
	}{
		{name: "BadRequest", handler: func(c *Context) { c.BadRequest("bad") }, status: http.StatusBadRequest},
		{name: "NotFound", handler: func(c *Context) { c.NotFound("missing") }, status: http.StatusNotFound},
		{name: "Unauthorized", handler: func(c *Context) { c.Unauthorized("auth") }, status: http.StatusUnauthorized},
		{name: "InternalError", handler: func(c *Context) { c.InternalError("boom") }, status: http.StatusInternalServerError},
		{name: "BadRequestWrap", handler: func(c *Context) { c.BadRequestWrap(stderrorsNew("boom"), "bad input") }, status: http.StatusBadRequest},
		{name: "NotFoundWrap", handler: func(c *Context) { c.NotFoundWrap(stderrorsNew("missing"), "user") }, status: http.StatusNotFound},
		{name: "InternalErrorWrap", handler: func(c *Context) { c.InternalErrorWrap(stderrorsNew("db"), "save", nil) }, status: http.StatusInternalServerError},
		{name: "ValidationError", handler: func(c *Context) { c.ValidationError(ErrBodyRequired) }, status: http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := New()
			a.Use(func(c *Context) {
				c.Set(CtxKeyRequestID, "trace-abc")
				c.Header("X-Request-ID", "trace-abc")
				c.Next()
			})
			a.Get("/x", tc.handler)

			w := httptest.NewRecorder()
			a.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
			require.Equal(t, tc.status, w.Code)

			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			errObj, ok := resp["error"].(map[string]any)
			require.True(t, ok, "missing error block: %s", w.Body.String())
			assert.Equal(t, "trace-abc", errObj["traceId"], "%s should carry traceId", tc.name)
		})
	}
	_ = app // ensure outer app doesn't get GC'd in test scope
}

func stderrorsNew(s string) error {
	return stderrors.New(s)
}

func TestSchemaUsage_AutoTagged(t *testing.T) {
	app := New()

	app.Post("/users", func(c *Context) {
		req, ok := BindAndValidate[schemaUsageRequest](c)
		if !ok {
			return
		}
		c.Created(schemaUsageResponse{ID: 1, Name: req.Name})
	})

	app.Get("/users", func(c *Context) {
		_, ok := BindQueryAndValidate[schemaUsageQuery](c)
		if !ok {
			return
		}
		c.Success([]schemaUsageResponse{{ID: 1, Name: "alice"}})
	})

	app.Get("/users/:id", func(c *Context) {
		_, ok := BindPathAndValidate[schemaUsagePath](c)
		if !ok {
			return
		}
		c.Success(schemaUsageResponse{ID: 1, Name: "alice"})
	})

	body := `{"name":"alice"}`
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body)),
		httptest.NewRequest(http.MethodGet, "/users?page=2", nil),
		httptest.NewRequest(http.MethodGet, "/users/1", nil),
	} {
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		require.True(t, w.Code >= 200 && w.Code < 300, "status %d", w.Code)
	}

	schemas := app.Schemas()
	byName := map[string][]string{}
	for _, s := range schemas {
		byName[s.Name] = s.Usage
	}

	assert.ElementsMatch(t, []string{"request"}, byName["schemaUsageRequest"])
	assert.ElementsMatch(t, []string{"query"}, byName["schemaUsageQuery"])
	assert.ElementsMatch(t, []string{"path"}, byName["schemaUsagePath"])
	assert.ElementsMatch(t, []string{"response"}, byName["schemaUsageResponse"],
		"response DTO should be auto-discovered from c.Success / c.Created without manual RegisterSchema")
}

func TestSchemaUsage_MergeOnRegisterAndBind(t *testing.T) {
	// 同一类型既被 RegisterSchema (response) 又作为请求体 (request) 使用：两个 usage 都应留下。
	app := New()
	app.RegisterSchema(schemaUsageRequest{})

	app.Post("/x", func(c *Context) {
		req, ok := BindAndValidate[schemaUsageRequest](c)
		if !ok {
			return
		}
		c.Success(req)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"name":"bob"}`))
	r.Header.Set("Content-Type", "application/json")
	app.Router.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	for _, s := range app.Schemas() {
		if s.Name == "schemaUsageRequest" {
			assert.ElementsMatch(t, []string{"request", "response"}, s.Usage)
			return
		}
	}
	t.Fatal("schemaUsageRequest not found")
}

func TestContext_Error(t *testing.T) {
	app := New()

	app.Get("/error", func(c *Context) {
		c.Error(http.StatusBadGateway, "BAD_GATEWAY", "upstream error")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)

	var resp H
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "BAD_GATEWAY", errObj["code"])
	assert.Equal(t, "upstream error", errObj["message"])
}
