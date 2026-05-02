package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Generate(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, err := client.Generate(1, "testuser", "admin")
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, "Bearer", tokens.TokenType)
	assert.NotZero(t, tokens.ExpiresIn)
}

func TestClient_Generate_DifferentUsers(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens1, _ := client.Generate(1, "user1", "user")
	tokens2, _ := client.Generate(2, "user2", "admin")

	assert.NotEqual(t, tokens1.AccessToken, tokens2.AccessToken)
}

func TestClient_Validate(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	// Generate token
	tokens, _ := client.Generate(42, "testuser", "user")

	// Validate token
	claims, err := client.Validate(tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "user", claims.Role)
}

func TestClient_Validate_InvalidToken(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	_, err := client.Validate("invalid-token")
	assert.Error(t, err)
}

func TestClient_Validate_WrongSecret(t *testing.T) {
	client1 := New(Config{
		SecretKey:     "secret-1",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	client2 := New(Config{
		SecretKey:     "secret-2",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client1.Generate(1, "user", "user")

	_, err := client2.Validate(tokens.AccessToken)
	assert.Error(t, err)
}

func TestClient_ValidateRefresh(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client.Generate(1, "testuser", "user")

	claims, err := client.ValidateRefresh(tokens.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, int64(1), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestClient_ValidateRefresh_WrongSecret(t *testing.T) {
	client1 := New(Config{
		SecretKey:     "secret-1",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	client2 := New(Config{
		SecretKey:     "secret-2",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client1.Generate(1, "user", "user")

	_, err := client2.ValidateRefresh(tokens.RefreshToken)
	assert.Error(t, err)
}

func TestClient_Refresh(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client.Generate(1, "testuser", "user")

	tokens2, err := client.Refresh(tokens.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens2.AccessToken)
	assert.NotEmpty(t, tokens2.RefreshToken)
}

func TestClient_Refresh_InvalidToken(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	_, err := client.Refresh("invalid-refresh-token")
	assert.Error(t, err)
}

func TestNew(t *testing.T) {
	client := New(Config{
		SecretKey:     "my-secret",
		Expiration:    time.Hour,
		RefreshExpiry: 24 * time.Hour,
	})

	assert.NotNil(t, client)
}

func TestClient_Generate_WithDifferentRoles(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	roles := []string{"user", "admin", "superadmin"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			tokens, err := client.Generate(1, "user", role)
			require.NoError(t, err)

			claims, err := client.Validate(tokens.AccessToken)
			require.NoError(t, err)
			assert.Equal(t, role, claims.Role)
		})
	}
}

func TestClaims_Fields(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client.Generate(100, "testuser", "admin")
	claims, _ := client.Validate(tokens.AccessToken)

	assert.Equal(t, "igo", claims.Issuer)
	assert.Equal(t, int64(100), claims.UserID)
}

func TestJWTMiddleware(t *testing.T) {
	client := New(Config{SecretKey: "test-secret-key", Expiration: time.Hour, RefreshExpiry: 24 * time.Hour})
	middleware := JWTMiddleware(client)

	assert.True(t, middleware(&Claims{UserID: 1, Username: "alice"}))
	assert.True(t, middleware(nil))
}
