package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config JWT 配置
type Config struct {
	SecretKey     string        // 密钥
	Expiration    time.Duration // 过期时间
	RefreshExpiry time.Duration // 刷新过期时间
}

// Claims JWT Claims
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// TokenResponse Token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Client JWT 客户端
type Client struct {
	cfg           Config
	refreshSecret []byte
}

// New 创建 JWT 客户端
func New(cfg Config) *Client {
	return &Client{
		cfg:           cfg,
		refreshSecret: []byte(cfg.SecretKey + "_refresh"),
	}
}

// Generate 生成 Token 对
func (c *Client) Generate(userID int64, username, role string) (*TokenResponse, error) {
	// Access Token
	accessClaims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(c.cfg.Expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "igo",
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString([]byte(c.cfg.SecretKey))
	if err != nil {
		return nil, fmt.Errorf("sign access token failed: %w", err)
	}

	// Refresh Token (更长过期时间)
	refreshClaims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(c.cfg.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "igo",
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString(c.refreshSecret)
	if err != nil {
		return nil, fmt.Errorf("sign refresh token failed: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresIn:    int64(c.cfg.Expiration.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// Validate 验证 Access Token
func (c *Client) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(c.cfg.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ValidateRefresh 验证 Refresh Token
func (c *Client) ValidateRefresh(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return c.refreshSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse refresh token failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}

	return claims, nil
}

// Refresh 使用 Refresh Token 刷新 Access Token
func (c *Client) Refresh(refreshToken string) (*TokenResponse, error) {
	claims, err := c.ValidateRefresh(refreshToken)
	if err != nil {
		return nil, err
	}

	return c.Generate(claims.UserID, claims.Username, claims.Role)
}

// JWTMiddleware JWT 中间件
func JWTMiddleware(client *Client) func(*Claims) bool {
	return func(claims *Claims) bool {
		return true
	}
}

// ErrInvalidToken 无效 Token 错误
var ErrInvalidToken = errors.New("invalid token")

// ErrExpiredToken 过期 Token 错误
var ErrExpiredToken = errors.New("token has expired")
