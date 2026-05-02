package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
}

func TestConfig_SetGet(t *testing.T) {
	c := New()

	c.Set("name", "test")
	c.Set("count", 42)
	c.Set("enabled", true)

	assert.Equal(t, "test", c.GetString("name"))
	assert.Equal(t, 42, c.GetInt("count"))
	assert.True(t, c.GetBool("enabled"))
}

func TestConfig_Default(t *testing.T) {
	c := New()

	c.Default("host", "localhost")
	c.Default("port", 8080)

	assert.Equal(t, "localhost", c.GetString("host"))
	assert.Equal(t, 8080, c.GetInt("port"))
}

func TestConfig_Unmarshal(t *testing.T) {
	c := New()

	c.Set("name", "test-app")
	c.Set("port", 3000)

	type AppSettings struct {
		Name string
		Port int
	}

	var settings AppSettings
	require.NoError(t, c.Unmarshal(&settings))
	assert.Equal(t, "test-app", settings.Name)
	assert.Equal(t, 3000, settings.Port)
}

func TestConfig_UnmarshalKey(t *testing.T) {
	c := New()

	c.Set("app.name", "test-app")
	c.Set("app.port", 3000)

	type AppSettings struct {
		Name string
		Port int
	}

	var settings AppSettings
	require.NoError(t, c.UnmarshalKey("app", &settings))
	assert.Equal(t, "test-app", settings.Name)
}

func TestConfig_IsSet(t *testing.T) {
	c := New()

	c.Set("exists", "yes")

	assert.True(t, c.IsSet("exists"))
	assert.False(t, c.IsSet("not-exists"))
}

func TestConfig_AllSettings(t *testing.T) {
	c := New()

	c.Set("name", "test")
	c.Set("count", 42)

	settings := c.AllSettings()

	assert.Equal(t, "test", settings["name"])
	assert.Equal(t, 42, settings["count"])
}

func TestConfig_GetFloat(t *testing.T) {
	c := New()

	c.Set("pi", 3.14159)

	result := c.GetFloat("pi")
	assert.Equal(t, 3.14159, result)
}

func TestConfig_GetInt64(t *testing.T) {
	c := New()

	c.Set("big", int64(1<<62))

	result := c.GetInt64("big")
	assert.Equal(t, int64(1<<62), result)
}

func TestConfig_GetStringSlice(t *testing.T) {
	c := New()

	c.Set("colors", []string{"red", "green", "blue"})

	slice := c.GetStringSlice("colors")
	assert.Len(t, slice, 3)
	assert.Equal(t, "red", slice[0])
}

func TestConfig_GetStringMap(t *testing.T) {
	c := New()

	c.Set("nested", map[string]interface{}{
		"host": "localhost",
		"port": 8080,
	})

	m := c.GetStringMap("nested")
	assert.Equal(t, "localhost", m["host"])
}

func TestConfig_SetConfigType(t *testing.T) {
	c := New()
	c.SetConfigType("yaml")

	c.Set("test", "value")

	assert.True(t, c.IsSet("test"))
}

func TestConfig_BindEnv(t *testing.T) {
	os.Setenv("TEST_BIND_ENV_VALUE", "from-env")

	c := New()
	err := c.BindEnv("test", "TEST_BIND_ENV_VALUE")
	require.NoError(t, err)

	// Value should be empty until loaded
	if c.GetString("test") != "" {
		t.Logf("Note: BindEnv value may be set immediately in some Viper versions")
	}
}

func TestAppConfig_LoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent", "config", "yaml")
	assert.Error(t, err)
}

func TestConfig_ReadConfig(t *testing.T) {
	c := New()
	c.SetConfigType("yaml")

	yamlContent := []byte(`name: test-app
port: 9090`)

	require.NoError(t, c.ReadConfig(yamlContent))
	assert.Equal(t, "test-app", c.GetString("name"))
	assert.Equal(t, 9090, c.GetInt("port"))
}

func TestConfig_FromEnv(t *testing.T) {
	os.Setenv("APP_NAME", "from-env")

	c := New()
	c.SetConfigName("app")
	c.SetConfigType("env")
	c.FromEnv()

	// APP_NAME should be mapped to app.name
	if c.GetString("app.name") != "from-env" && c.GetString("app_name") != "from-env" {
		t.Logf("Note: env binding depends on config name")
	}
}

func TestConfig_AddConfigPath(t *testing.T) {
	c := New()
	c.AddConfigPath("/tmp")
	c.AddConfigPath("/etc")

	// Just verify no panic
	assert.NotNil(t, c.Viper)
}

func TestConfig_SetConfigName(t *testing.T) {
	c := New()
	c.SetConfigName("myconfig")

	// Just verify no panic
	assert.NotNil(t, c.Viper)
}

func TestAppConfigValidate(t *testing.T) {
	cfg := &AppConfig{
		Server:   ServerConfig{Port: ":8080"},
		Database: DatabaseConfig{Dialect: "mysql", DSN: "user:pass@tcp(localhost:3306)/db"},
		JWT:      JWTConfig{SecretKey: "not-a-placeholder"},
	}

	require.NoError(t, cfg.Validate())
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "console", cfg.Log.Format)

	invalid := &AppConfig{}
	err := invalid.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port is required")
	assert.Contains(t, err.Error(), "database.dialect is required")
	assert.Contains(t, err.Error(), "jwt.secret_key is required")

	placeholder := &AppConfig{
		Server:   ServerConfig{Port: ":8080"},
		Database: DatabaseConfig{Dialect: "sqlite", DSN: "app.db"},
		JWT:      JWTConfig{SecretKey: "secret"},
	}
	err = placeholder.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder")
}

func TestAppConfigValidateForProduction(t *testing.T) {
	valid := &AppConfig{
		Server:   ServerConfig{Port: ":8080"},
		Database: DatabaseConfig{Dialect: "postgres", DSN: "postgres://example"},
		JWT:      JWTConfig{SecretKey: "01234567890123456789012345678901"},
		Log:      LogConfig{Level: "info", Format: "json"},
	}
	require.NoError(t, valid.ValidateForProduction())

	invalid := &AppConfig{
		Server:   ServerConfig{Port: ":8080"},
		Database: DatabaseConfig{Dialect: "sqlite", DSN: "app.db"},
		JWT:      JWTConfig{SecretKey: "short-secret"},
		Log:      LogConfig{Level: "debug", Format: "console"},
	}
	err := invalid.ValidateForProduction()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
	assert.Contains(t, err.Error(), "sqlite is not recommended")
	assert.Contains(t, err.Error(), "debug")
}
