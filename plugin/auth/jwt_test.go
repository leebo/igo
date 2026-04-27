package auth

import (
	"testing"
	"time"
)

func TestClient_Generate(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, err := client.Generate(1, "testuser", "admin")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	if tokens.TokenType != "Bearer" {
		t.Errorf("expected token type 'Bearer', got '%s'", tokens.TokenType)
	}

	if tokens.ExpiresIn == 0 {
		t.Error("expected non-zero expires in")
	}
}

func TestClient_Generate_DifferentUsers(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens1, _ := client.Generate(1, "user1", "user")
	tokens2, _ := client.Generate(2, "user2", "admin")

	if tokens1.AccessToken == tokens2.AccessToken {
		t.Error("expected different tokens for different users")
	}
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
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", claims.UserID)
	}

	if claims.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got '%s'", claims.Username)
	}

	if claims.Role != "user" {
		t.Errorf("expected Role 'user', got '%s'", claims.Role)
	}
}

func TestClient_Validate_InvalidToken(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	_, err := client.Validate("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
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
	if err == nil {
		t.Error("expected error when validating with wrong secret")
	}
}

func TestClient_ValidateRefresh(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client.Generate(1, "testuser", "user")

	claims, err := client.ValidateRefresh(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefresh() error = %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", claims.UserID)
	}

	if claims.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got '%s'", claims.Username)
	}
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
	if err == nil {
		t.Error("expected error when validating refresh with wrong secret")
	}
}

func TestClient_Refresh(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	tokens, _ := client.Generate(1, "testuser", "user")

	tokens2, err := client.Refresh(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if tokens2.AccessToken == "" {
		t.Error("expected non-empty access token")
	}

	if tokens2.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
}

func TestClient_Refresh_InvalidToken(t *testing.T) {
	client := New(Config{
		SecretKey:     "test-secret-key",
		Expiration:    24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	})

	_, err := client.Refresh("invalid-refresh-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestNew(t *testing.T) {
	client := New(Config{
		SecretKey:     "my-secret",
		Expiration:    time.Hour,
		RefreshExpiry: 24 * time.Hour,
	})

	if client == nil {
		t.Error("expected non-nil client")
	}
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
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			claims, err := client.Validate(tokens.AccessToken)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if claims.Role != role {
				t.Errorf("expected Role '%s', got '%s'", role, claims.Role)
			}
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

	if claims.Issuer != "igo" {
		t.Errorf("expected Issuer 'igo', got '%s'", claims.Issuer)
	}

	if claims.UserID != 100 {
		t.Errorf("expected UserID 100, got %d", claims.UserID)
	}
}
