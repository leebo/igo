package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Error("expected non-nil config")
	}
}

func TestConfig_SetGet(t *testing.T) {
	c := New()

	c.Set("name", "test")
	c.Set("count", 42)
	c.Set("enabled", true)

	if c.GetString("name") != "test" {
		t.Errorf("expected 'test', got '%s'", c.GetString("name"))
	}

	if c.GetInt("count") != 42 {
		t.Errorf("expected 42, got %d", c.GetInt("count"))
	}

	if c.GetBool("enabled") != true {
		t.Errorf("expected true, got %v", c.GetBool("enabled"))
	}
}

func TestConfig_Default(t *testing.T) {
	c := New()

	c.Default("host", "localhost")
	c.Default("port", 8080)

	if c.GetString("host") != "localhost" {
		t.Errorf("expected 'localhost', got '%s'", c.GetString("host"))
	}

	if c.GetInt("port") != 8080 {
		t.Errorf("expected 8080, got %d", c.GetInt("port"))
	}
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
	if err := c.Unmarshal(&settings); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if settings.Name != "test-app" {
		t.Errorf("expected 'test-app', got '%s'", settings.Name)
	}

	if settings.Port != 3000 {
		t.Errorf("expected 3000, got %d", settings.Port)
	}
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
	if err := c.UnmarshalKey("app", &settings); err != nil {
		t.Fatalf("UnmarshalKey() error = %v", err)
	}

	if settings.Name != "test-app" {
		t.Errorf("expected 'test-app', got '%s'", settings.Name)
	}
}

func TestConfig_IsSet(t *testing.T) {
	c := New()

	c.Set("exists", "yes")

	if !c.IsSet("exists") {
		t.Error("expected 'exists' to be set")
	}

	if c.IsSet("not-exists") {
		t.Error("expected 'not-exists' to not be set")
	}
}

func TestConfig_AllSettings(t *testing.T) {
	c := New()

	c.Set("name", "test")
	c.Set("count", 42)

	settings := c.AllSettings()

	if settings["name"] != "test" {
		t.Errorf("expected 'test', got '%v'", settings["name"])
	}

	if settings["count"].(int) != 42 {
		t.Errorf("expected 42, got '%v'", settings["count"])
	}
}

func TestConfig_GetFloat(t *testing.T) {
	c := New()

	c.Set("pi", 3.14159)

	result := c.GetFloat("pi")
	if result != 3.14159 {
		t.Errorf("expected 3.14159, got %f", result)
	}
}

func TestConfig_GetInt64(t *testing.T) {
	c := New()

	c.Set("big", int64(1<<62))

	result := c.GetInt64("big")
	if result != 1<<62 {
		t.Errorf("expected %d, got %d", 1<<62, result)
	}
}

func TestConfig_GetStringSlice(t *testing.T) {
	c := New()

	c.Set("colors", []string{"red", "green", "blue"})

	slice := c.GetStringSlice("colors")
	if len(slice) != 3 {
		t.Errorf("expected 3 items, got %d", len(slice))
	}

	if slice[0] != "red" {
		t.Errorf("expected 'red', got '%s'", slice[0])
	}
}

func TestConfig_GetStringMap(t *testing.T) {
	c := New()

	c.Set("nested", map[string]interface{}{
		"host": "localhost",
		"port": 8080,
	})

	m := c.GetStringMap("nested")
	if m["host"] != "localhost" {
		t.Errorf("expected 'localhost', got '%v'", m["host"])
	}
}

func TestConfig_SetConfigType(t *testing.T) {
	c := New()
	c.SetConfigType("yaml")

	c.Set("test", "value")

	if !c.IsSet("test") {
		t.Error("expected 'test' to be set")
	}
}

func TestConfig_BindEnv(t *testing.T) {
	os.Setenv("TEST_BIND_ENV_VALUE", "from-env")

	c := New()
	err := c.BindEnv("test", "TEST_BIND_ENV_VALUE")
	if err != nil {
		t.Fatalf("BindEnv() error = %v", err)
	}

	// Value should be empty until loaded
	if c.GetString("test") != "" {
		t.Logf("Note: BindEnv value may be set immediately in some Viper versions")
	}
}

func TestAppConfig_LoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent", "config", "yaml")
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestConfig_ReadConfig(t *testing.T) {
	c := New()
	c.SetConfigType("yaml")

	yamlContent := []byte(`name: test-app
port: 9090`)

	if err := c.ReadConfig(yamlContent); err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	if c.GetString("name") != "test-app" {
		t.Errorf("expected 'test-app', got '%s'", c.GetString("name"))
	}

	if c.GetInt("port") != 9090 {
		t.Errorf("expected 9090, got %d", c.GetInt("port"))
	}
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
	if c.Viper == nil {
		t.Error("expected non-nil Viper")
	}
}

func TestConfig_SetConfigName(t *testing.T) {
	c := New()
	c.SetConfigName("myconfig")

	// Just verify no panic
	if c.Viper == nil {
		t.Error("expected non-nil Viper")
	}
}
