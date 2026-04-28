// Package main 演示完整的 JWT 认证流程：
//
//	register → login → 受保护路由 → refresh
//
// AI 学习要点：
//   - 用 bcrypt 哈希密码（绝不能存明文）
//   - access token 短期（15min），refresh token 长期（7d）
//   - 用 igo Group 把"需认证"的路由统一挂上 authMiddleware
//   - middleware 用 c.GinContextData 把 Claims 注入到 ctx，handler 通过 helper 取
//   - 认证失败短路时必须 return，不能继续 c.Next()
//
// 测试：
//
//	curl -X POST http://localhost:8080/auth/register -d '{"username":"alice","password":"secret123"}'
//	curl -X POST http://localhost:8080/auth/login    -d '{"username":"alice","password":"secret123"}'
//	# 拿到 access_token 后：
//	curl -H "Authorization: Bearer <access_token>" http://localhost:8080/me
//	curl -X POST http://localhost:8080/auth/refresh -d '{"refresh_token":"<refresh_token>"}'
package main

import (
	"errors"
	"strings"
	"sync"
	"time"

	igo "github.com/igo/igo"
	"github.com/igo/igo/core"
	"github.com/igo/igo/plugin/auth"

	"golang.org/x/crypto/bcrypt"
)

// =============================================================================
// 用户存储（内存模拟，生产应换成 DB）
// =============================================================================

type storedUser struct {
	ID           int64
	Username     string
	PasswordHash []byte // bcrypt 哈希后的密码
	Role         string
}

type userStore struct {
	mu    sync.Mutex
	byID  map[int64]*storedUser
	byUN  map[string]*storedUser
	nextID int64
}

func newUserStore() *userStore {
	return &userStore{
		byID:   make(map[int64]*storedUser),
		byUN:   make(map[string]*storedUser),
		nextID: 1,
	}
}

func (s *userStore) Create(username, password string) (*storedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byUN[username]; ok {
		return nil, errors.New("username already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &storedUser{
		ID:           s.nextID,
		Username:     username,
		PasswordHash: hash,
		Role:         "user",
	}
	s.byID[u.ID] = u
	s.byUN[u.Username] = u
	s.nextID++
	return u, nil
}

func (s *userStore) FindByUsername(username string) (*storedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byUN[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

// =============================================================================
// 认证中间件：解析 Bearer token，把 Claims 注入到 Context
// =============================================================================

const claimsKey = "auth.claims"

func authMiddleware(client *auth.Client) core.MiddlewareFunc {
	return func(c *core.Context) {
		header := c.Request.Header.Get("Authorization")
		if header == "" {
			c.Unauthorized("Authorization header required")
			return
		}
		// 期望格式：Bearer <token>
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			c.Unauthorized("expected 'Bearer <token>' format")
			return
		}
		claims, err := client.Validate(token)
		if err != nil {
			// 不要把内部错误细节暴露给客户端
			c.Unauthorized("invalid or expired token")
			return
		}
		c.GinContextData[claimsKey] = claims
		c.Next()
	}
}

// claimsFrom 从 Context 取出当前请求的 Claims（必须已经过 authMiddleware）
func claimsFrom(c *core.Context) *auth.Claims {
	v, ok := c.GinContextData[claimsKey]
	if !ok {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
}

// =============================================================================
// 请求/响应类型
// =============================================================================

type RegisterRequest struct {
	Username string `json:"username" validate:"required|min:3|max:50"`
	Password string `json:"password" validate:"required|min:8|max:72"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// =============================================================================
// Handlers
// =============================================================================

func registerHandler(store *userStore) core.HandlerFunc {
	return func(c *core.Context) {
		req, ok := igo.BindAndValidate[RegisterRequest](c)
		if !ok {
			return
		}
		u, err := store.Create(req.Username, req.Password)
		if err != nil {
			c.BadRequestWrap(err, "registration failed")
			return
		}
		c.Created(core.H{
			"id":       u.ID,
			"username": u.Username,
		})
	}
}

func loginHandler(store *userStore, client *auth.Client) core.HandlerFunc {
	return func(c *core.Context) {
		req, ok := igo.BindAndValidate[LoginRequest](c)
		if !ok {
			return
		}
		u, err := store.FindByUsername(req.Username)
		if err != nil {
			// 故意用相同的 401 消息，避免泄露"用户名是否存在"
			c.Unauthorized("invalid credentials")
			return
		}
		if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(req.Password)); err != nil {
			c.Unauthorized("invalid credentials")
			return
		}
		tokens, err := client.Generate(u.ID, u.Username, u.Role)
		if err != nil {
			c.InternalErrorWrap(err, "failed to generate token", nil)
			return
		}
		c.Success(tokens)
	}
}

func refreshHandler(client *auth.Client) core.HandlerFunc {
	return func(c *core.Context) {
		req, ok := igo.BindAndValidate[RefreshRequest](c)
		if !ok {
			return
		}
		tokens, err := client.Refresh(req.RefreshToken)
		if err != nil {
			c.Unauthorized("invalid or expired refresh token")
			return
		}
		c.Success(tokens)
	}
}

// meHandler 受保护路由：返回当前登录用户
func meHandler(c *core.Context) {
	claims := claimsFrom(c)
	if claims == nil {
		// 理论上不会走到这里：authMiddleware 已挡掉
		c.Unauthorized("not authenticated")
		return
	}
	c.Success(core.H{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
	})
}

// =============================================================================
// main
// =============================================================================

// setupApp 把 store/client 注入构造完整 App，便于测试
func setupApp(store *userStore, client *auth.Client) *igo.App {
	app := igo.Simple()

	// 公开 API
	app.Group("/auth", func(a *igo.App) {
		a.Post("/register", registerHandler(store))
		a.Post("/login", loginHandler(store, client))
		a.Post("/refresh", refreshHandler(client))
	})

	// 受保护 API（中间件作为 Group 第三参数，不要在闭包里 Use）
	app.Group("", func(p *igo.App) {
		p.Get("/me", meHandler)
	}, authMiddleware(client))

	return app
}

func main() {
	store := newUserStore()
	client := auth.New(auth.Config{
		// ⚠️ 演示用密钥，生产请从环境变量/Secret Manager 读取
		SecretKey:     "demo-secret-please-change",
		Expiration:    15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	app := setupApp(store, client)
	app.PrintRoutes()
	app.Run(":8080")
}
