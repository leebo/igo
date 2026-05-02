package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	igo "github.com/leebo/igo"
	"github.com/leebo/igo/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock：实现 UserService 接口，可在测试中注入任意行为
// =============================================================================

type mockUserService struct {
	users     map[int64]*User
	getErr    error
	createErr error
	created   *User // 记录最后一次 Create 调用
}

func (m *mockUserService) GetByID(id int64) (*User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserService) Create(name string) (*User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	u := &User{ID: 99, Name: name}
	m.created = u
	return u, nil
}

// =============================================================================
// 辅助：构造一个挂好 handler 的 App，并执行单次请求
// =============================================================================

func runRequest(t *testing.T, app *igo.App, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	var req *http.Request
	if rdr != nil {
		req = httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

// decodeBody 反序列化 JSON 响应到 map
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

// =============================================================================
// 表驱动测试：GET /users/:id
// 覆盖：成功 / 404 / 参数无效 / service 故障
// =============================================================================

func TestUserHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		users      map[int64]*User
		getErr     error
		wantStatus int
		wantBody   string // body 应包含的子串
	}{
		{
			name:       "happy path",
			path:       "/users/1",
			users:      map[int64]*User{1: {ID: 1, Name: "Alice"}},
			wantStatus: 200,
			wantBody:   `"name":"Alice"`,
		},
		{
			name:       "not found",
			path:       "/users/99",
			users:      map[int64]*User{},
			wantStatus: 404,
			wantBody:   "user not found",
		},
		{
			name:       "invalid id",
			path:       "/users/abc",
			users:      map[int64]*User{},
			wantStatus: 400,
			wantBody:   "invalid parameter",
		},
		{
			name:       "service failure",
			path:       "/users/1",
			users:      map[int64]*User{},
			getErr:     errors.New("db down"),
			wantStatus: 404, // handler 把所有 err 当 not found 处理（演示）
			wantBody:   "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := igo.New()
			h := newUserHandler(&mockUserService{users: tt.users, getErr: tt.getErr})
			app.Get("/users/:id", h.Get)

			w := runRequest(t, app, http.MethodGet, tt.path, nil)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}

// =============================================================================
// 测试 POST 请求体绑定 + 校验
// =============================================================================

func TestUserHandler_Create(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		createErr   error
		wantStatus  int
		wantContain string
	}{
		{
			name:        "happy path",
			body:        `{"name":"Bob"}`,
			wantStatus:  201,
			wantContain: `"name":"Bob"`,
		},
		{
			name:        "validation: name too short",
			body:        `{"name":"X"}`,
			wantStatus:  422, // BindAndValidate 自动 422
			wantContain: "VALIDATION_FAILED",
		},
		{
			name:        "validation: name missing",
			body:        `{}`,
			wantStatus:  422,
			wantContain: "VALIDATION_FAILED",
		},
		{
			name:        "invalid json",
			body:        `not-json`,
			wantStatus:  400,
			wantContain: "BAD_REQUEST",
		},
		{
			name:        "service failure",
			body:        `{"name":"Bob"}`,
			createErr:   errors.New("storage offline"),
			wantStatus:  500,
			wantContain: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := igo.New()
			mock := &mockUserService{createErr: tt.createErr}
			h := newUserHandler(mock)
			app.Post("/users", h.Create)

			w := runRequest(t, app, http.MethodPost, "/users", []byte(tt.body))

			assert.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantContain)
		})
	}
}

// =============================================================================
// 验证 happy path 时 mock 被正确调用（行为断言）
// =============================================================================

func TestUserHandler_Create_CallsService(t *testing.T) {
	app := igo.New()
	mock := &mockUserService{}
	h := newUserHandler(mock)
	app.Post("/users", h.Create)

	runRequest(t, app, http.MethodPost, "/users", []byte(`{"name":"Charlie"}`))

	require.NotNil(t, mock.created)
	assert.Equal(t, "Charlie", mock.created.Name)
}

// =============================================================================
// 测试中间件：验证它正确调用了 c.Next() 并设置了响应头
// =============================================================================

func TestMiddleware_AddsHeader(t *testing.T) {
	app := igo.New()

	// 一个简单的中间件：在响应里加 header
	app.Use(func(c *core.Context) {
		c.Header("X-Trace-ID", "trace-123")
		c.Next()
	})
	app.Get("/ping", func(c *core.Context) {
		c.Success(core.H{"pong": true})
	})

	w := runRequest(t, app, http.MethodGet, "/ping", nil)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "trace-123", w.Header().Get("X-Trace-ID"))

	body := decodeBody(t, w)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, data["pong"])
}

// =============================================================================
// 测试中间件短路：未授权请求应当被中间件拦截，handler 不应执行
// =============================================================================

func TestMiddleware_ShortCircuits(t *testing.T) {
	app := igo.New()
	handlerCalled := false

	app.Use(func(c *core.Context) {
		if c.Request.Header.Get("X-Auth") == "" {
			c.Unauthorized("missing token")
			return // 关键：不调 c.Next()
		}
		c.Next()
	})
	app.Get("/protected", func(c *core.Context) {
		handlerCalled = true
		c.Success(core.H{"ok": true})
	})

	w := runRequest(t, app, http.MethodGet, "/protected", nil)

	assert.Equal(t, 401, w.Code)
	assert.False(t, handlerCalled)
}
