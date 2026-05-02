package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	igo "github.com/leebo/igo"
	"github.com/leebo/igo/plugin/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 测试夹具
// =============================================================================

func newTestApp(t *testing.T) (*igo.App, *userStore, *auth.Client) {
	t.Helper()
	store := newUserStore()
	client := auth.New(auth.Config{
		SecretKey:     "test-secret-very-long-just-for-testing",
		Expiration:    1 * time.Hour,
		RefreshExpiry: 24 * time.Hour,
	})
	return setupApp(store, client), store, client
}

// post 发起一次 JSON POST 请求
func post(t *testing.T, app *igo.App, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

// getWithToken 带 Authorization header 的 GET
func getWithToken(t *testing.T, app *igo.App, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

// decodeData 把响应里的 data 字段反序列化到 dst
func decodeData(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper), w.Body.String())
	require.NoError(t, json.Unmarshal(wrapper.Data, dst), string(wrapper.Data))
}

// =============================================================================
// register
// =============================================================================

func TestRegister_Success(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/register", map[string]string{
		"username": "alice",
		"password": "secret123",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	decodeData(t, w, &resp)
	assert.Equal(t, "alice", resp.Username)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck
	w := post(t, app, "/auth/register", map[string]string{
		"username": "alice",
		"password": "another-pass",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "already exists")
}

func TestRegister_PasswordTooShort(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/register", map[string]string{
		"username": "bob",
		"password": "x", // 远低于 min:8
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	// 应当返回结构化错误，带 field/rule
	assert.Contains(t, w.Body.String(), `"field":"Password"`)
}

// =============================================================================
// login
// =============================================================================

func TestLogin_Success(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice",
		"password": "secret123",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var tokens auth.TokenResponse
	decodeData(t, w, &tokens)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, "Bearer", tokens.TokenType)
}

func TestLogin_WrongPassword(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong-password",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	// 关键：响应消息不应泄露 "用户名是否存在"
	assert.Contains(t, w.Body.String(), "invalid credentials")
}

func TestLogin_NonexistentUser(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/login", map[string]string{
		"username": "ghost",
		"password": "any",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// =============================================================================
// 受保护路由
// =============================================================================

func TestProtectedRoute_WithValidToken(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	// 1) login 拿 access token
	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice",
		"password": "secret123",
	})
	var tokens auth.TokenResponse
	decodeData(t, w, &tokens)

	// 2) 用 token 访问 /me
	w = getWithToken(t, app, "/me", tokens.AccessToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var me struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	decodeData(t, w, &me)
	assert.Equal(t, "alice", me.Username)
	assert.Equal(t, "user", me.Role)
}

func TestProtectedRoute_NoToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := getWithToken(t, app, "/me", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")
}

func TestProtectedRoute_BadFormat(t *testing.T) {
	app, _, _ := newTestApp(t)
	// 缺少 Bearer 前缀
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "abc.def.ghi")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Bearer")
}

func TestProtectedRoute_InvalidToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := getWithToken(t, app, "/me", "garbage.token.value")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// =============================================================================
// refresh
// =============================================================================

func TestRefresh_Success(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	// 1) login 拿 token 对
	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice", "password": "secret123",
	})
	var tokens auth.TokenResponse
	decodeData(t, w, &tokens)

	// 2) refresh 拿一对新 token
	w = post(t, app, "/auth/refresh", map[string]string{
		"refresh_token": tokens.RefreshToken,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var newTokens auth.TokenResponse
	decodeData(t, w, &newTokens)
	assert.NotEmpty(t, newTokens.AccessToken)
	// 新的 access_token 应该能用
	w = getWithToken(t, app, "/me", newTokens.AccessToken)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestRefresh_InvalidToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/refresh", map[string]string{
		"refresh_token": "not-a-real-token",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

func TestRefresh_AccessTokenIsRejected(t *testing.T) {
	// access token 不能用作 refresh（有不同的密钥）
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice", "password": "secret123",
	})
	var tokens auth.TokenResponse
	decodeData(t, w, &tokens)

	w = post(t, app, "/auth/refresh", map[string]string{
		"refresh_token": tokens.AccessToken, // 故意用 access token
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
