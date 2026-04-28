package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	igo "github.com/leebo/igo"
	"github.com/leebo/igo/plugin/auth"
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
	raw, _ := json.Marshal(body)
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
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("decode wrapper: %v\nbody=%s", err, w.Body.String())
	}
	if err := json.Unmarshal(wrapper.Data, dst); err != nil {
		t.Fatalf("decode data: %v\nraw=%s", err, wrapper.Data)
	}
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
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	decodeData(t, w, &resp)
	if resp.Username != "alice" {
		t.Errorf("username = %q, want alice", resp.Username)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck
	w := post(t, app, "/auth/register", map[string]string{
		"username": "alice",
		"password": "another-pass",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "already exists") {
		t.Errorf("body should mention duplicate: %s", w.Body.String())
	}
}

func TestRegister_PasswordTooShort(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/register", map[string]string{
		"username": "bob",
		"password": "x", // 远低于 min:8
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body=%s", w.Code, w.Body.String())
	}
	// 应当返回结构化错误，带 field/rule
	if !strings.Contains(w.Body.String(), `"field":"Password"`) {
		t.Errorf("body should include field=Password: %s", w.Body.String())
	}
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
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var tokens auth.TokenResponse
	decodeData(t, w, &tokens)
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Errorf("expected non-empty tokens, got %+v", tokens)
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", tokens.TokenType)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	app, store, _ := newTestApp(t)
	store.Create("alice", "secret123") //nolint:errcheck

	w := post(t, app, "/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong-password",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	// 关键：响应消息不应泄露 "用户名是否存在"
	if !strings.Contains(w.Body.String(), "invalid credentials") {
		t.Errorf("body should generic-error: %s", w.Body.String())
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/login", map[string]string{
		"username": "ghost",
		"password": "any",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
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
	if w.Code != http.StatusOK {
		t.Fatalf("/me status = %d, body=%s", w.Code, w.Body.String())
	}

	var me struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	decodeData(t, w, &me)
	if me.Username != "alice" {
		t.Errorf("username = %q, want alice", me.Username)
	}
	if me.Role != "user" {
		t.Errorf("role = %q, want user", me.Role)
	}
}

func TestProtectedRoute_NoToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := getWithToken(t, app, "/me", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Authorization header required") {
		t.Errorf("body should explain missing header: %s", w.Body.String())
	}
}

func TestProtectedRoute_BadFormat(t *testing.T) {
	app, _, _ := newTestApp(t)
	// 缺少 Bearer 前缀
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "abc.def.ghi")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Bearer") {
		t.Errorf("body should mention Bearer format: %s", w.Body.String())
	}
}

func TestProtectedRoute_InvalidToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := getWithToken(t, app, "/me", "garbage.token.value")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
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
	if w.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body=%s", w.Code, w.Body.String())
	}

	var newTokens auth.TokenResponse
	decodeData(t, w, &newTokens)
	if newTokens.AccessToken == "" {
		t.Error("expected non-empty access_token after refresh")
	}
	// 新的 access_token 应该能用
	w = getWithToken(t, app, "/me", newTokens.AccessToken)
	if w.Code != http.StatusOK {
		t.Errorf("new access_token doesn't work; status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	app, _, _ := newTestApp(t)
	w := post(t, app, "/auth/refresh", map[string]string{
		"refresh_token": "not-a-real-token",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
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
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (access token must not be valid as refresh)", w.Code)
	}
}
