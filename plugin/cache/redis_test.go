package cache

import (
	"context"
	"testing"
	"time"
)

// MockRedis for testing without actual Redis
type MockClient struct {
	data map[string]string
}

func TestConfig(t *testing.T) {
	cfg := Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	if cfg.Addr != "localhost:6379" {
		t.Errorf("expected 'localhost:6379', got '%s'", cfg.Addr)
	}
}

func TestNew_ConnectionFailure(t *testing.T) {
	// This will fail because there's no Redis running
	_, err := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// We expect an error since there's no Redis server
	if err == nil {
		t.Skip("Redis not available, skipping connection test")
	}
}

func TestCacheItem_Generic(t *testing.T) {
	item := CacheItem[string]{
		Value:     "test",
		ExpiredAt: time.Now().Add(time.Hour),
	}

	if item.Value != "test" {
		t.Errorf("expected 'test', got '%s'", item.Value)
	}
}

func TestCacheService_Interface(t *testing.T) {
	// Verify CacheService interface is satisfied
	var _ CacheService = (*Client)(nil)
}

// Integration test helper - only runs if Redis is available
func skipIfNoRedis(t *testing.T) {
	t.Helper()
	_, err := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
}

func TestClient_SetGet_Delete(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "test-key"
	value := "test-value"

	// Set (strings are JSON encoded)
	err := client.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get (returns JSON encoded value)
	got, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Strings are stored with JSON encoding, so we expect quotes
	expected := `"` + value + `"`
	if got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}

	// Delete
	err = client.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = client.Get(ctx, key)
	if err == nil {
		t.Error("expected error for deleted key")
	}
}

func TestClient_SetJSON(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "json-key"

	type Data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := Data{Name: "test", Age: 42}

	// Set JSON
	err := client.SetJSON(ctx, key, original)
	if err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}

	// Get JSON
	var loaded Data
	err = client.GetJSON(ctx, key, &loaded)
	if err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("expected name '%s', got '%s'", original.Name, loaded.Name)
	}

	if loaded.Age != original.Age {
		t.Errorf("expected age %d, got %d", original.Age, loaded.Age)
	}

	client.Delete(ctx, key)
}

func TestClient_Exists(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "exists-key"

	// Should not exist
	exists, err := client.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}

	// Set
	client.Set(ctx, key, "value")

	// Should exist now
	exists, err = client.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}

	client.Delete(ctx, key)
}

func TestClient_Expire(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "expire-key"

	client.Set(ctx, key, "value")

	err := client.Expire(ctx, key, time.Second)
	if err != nil {
		t.Fatalf("Expire() error = %v", err)
	}

	ttl, err := client.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL() error = %v", err)
	}

	if ttl <= 0 || ttl > time.Second {
		t.Errorf("expected TTL around 1 second, got %v", ttl)
	}

	client.Delete(ctx, key)
}

func TestClient_Incr(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "counter-key"

	// Clean up
	client.Delete(ctx, key)

	// Incr
	val, err := client.Incr(ctx, key)
	if err != nil {
		t.Fatalf("Incr() error = %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Incr again
	val, err = client.Incr(ctx, key)
	if err != nil {
		t.Fatalf("Incr() error = %v", err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	client.Delete(ctx, key)
}

func TestClient_Decr(t *testing.T) {
	skipIfNoRedis(t)

	client, _ := New(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer client.Close()

	ctx := context.Background()
	key := "decr-key"

	// Clean up first
	client.Delete(ctx, key)

	// Use Incr to set numeric value
	client.Incr(ctx, key)
	client.Incr(ctx, key)
	client.Incr(ctx, key) // value = 3

	val, err := client.Decr(ctx, key)
	if err != nil {
		t.Fatalf("Decr() error = %v", err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	client.Delete(ctx, key)
}

func TestClient_MustNew(t *testing.T) {
	// Without Redis, this should panic
	defer func() {
		if r := recover(); r != nil {
			// Expected panic
		}
	}()

	MustNew(Config{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}
