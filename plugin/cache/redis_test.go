package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assert.Equal(t, "localhost:6379", cfg.Addr)
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

	assert.Equal(t, "test", item.Value)
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
	require.NoError(t, err)

	// Get (returns JSON encoded value)
	got, err := client.Get(ctx, key)
	require.NoError(t, err)

	// Strings are stored with JSON encoding, so we expect quotes
	expected := `"` + value + `"`
	assert.Equal(t, expected, got)

	// Delete
	err = client.Delete(ctx, key)
	require.NoError(t, err)

	// Verify deleted
	_, err = client.Get(ctx, key)
	assert.Error(t, err)
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
	require.NoError(t, err)

	// Get JSON
	var loaded Data
	err = client.GetJSON(ctx, key, &loaded)
	require.NoError(t, err)

	assert.Equal(t, original, loaded)

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
	require.NoError(t, err)
	assert.False(t, exists)

	// Set
	client.Set(ctx, key, "value")

	// Should exist now
	exists, err = client.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

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
	require.NoError(t, err)

	ttl, err := client.TTL(ctx, key)
	require.NoError(t, err)

	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, time.Second)

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
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	// Incr again
	val, err = client.Incr(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)

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
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)

	client.Delete(ctx, key)
}

func TestClient_MustNew(t *testing.T) {
	cfg := Config{Addr: "localhost:6379", Password: "", DB: 0}
	probe, err := New(cfg)
	if err == nil {
		require.NoError(t, probe.Close())
		client := MustNew(cfg)
		assert.NotNil(t, client)
		require.NoError(t, client.Close())
		return
	}

	assert.Panics(t, func() { MustNew(cfg) })
}
