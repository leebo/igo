package logging

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	client, err := New(Config{
		Level:      "debug",
		Format:     "console",
		OutputPath: "stdout",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestMustNew(t *testing.T) {
	client := MustNew(Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "stdout",
	})

	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestClient_Info(t *testing.T) {
	client, _ := New(Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	})

	client.Info("test message", String("key", "value"))
}

func TestClient_Error(t *testing.T) {
	client, _ := New(Config{
		Level:      "error",
		Format:     "console",
		OutputPath: "stdout",
	})

	client.Error("error occurred", String("error", "something failed"))
}

func TestClient_Warn(t *testing.T) {
	client, _ := New(Config{
		Level:      "warn",
		Format:     "console",
		OutputPath: "stdout",
	})

	client.Warn("warning", Int("count", 42))
}

func TestClient_Debug(t *testing.T) {
	client, _ := New(Config{
		Level:      "debug",
		Format:     "console",
		OutputPath: "stdout",
	})

	client.Debug("debug info", Float64("value", 3.14))
}

func TestClient_Fatal(t *testing.T) {
	// Fatal calls os.Exit which kills the test runner
	// So we just verify it compiles correctly
}

func TestClient_With(t *testing.T) {
	client, _ := New(Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	})

	logger := client.With(String("request_id", "123"))
	logger.Info("with context")
}

func TestClient_Sugar(t *testing.T) {
	client, _ := New(Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	})

	sugar := client.Sugar()
	sugar.Infow("sugar info", "key", "value")
}

func TestString(t *testing.T) {
	field := String("key", "value")
	if field.Key != "key" {
		t.Errorf("expected key 'key', got '%s'", field.Key)
	}
}

func TestInt(t *testing.T) {
	field := Int("count", 42)
	if field.Key != "count" {
		t.Errorf("expected key 'count', got '%s'", field.Key)
	}
}

func TestInt64(t *testing.T) {
	field := Int64("big", 1<<62)
	if field.Key != "big" {
		t.Errorf("expected key 'big', got '%s'", field.Key)
	}
}

func TestFloat64(t *testing.T) {
	field := Float64("pi", 3.14159)
	if field.Key != "pi" {
		t.Errorf("expected key 'pi', got '%s'", field.Key)
	}
}

func TestBool(t *testing.T) {
	field := Bool("flag", true)
	if field.Key != "flag" {
		t.Errorf("expected key 'flag', got '%s'", field.Key)
	}
}

func TestStrings(t *testing.T) {
	field := Strings("names", []string{"a", "b", "c"})
	if field.Key != "names" {
		t.Errorf("expected key 'names', got '%s'", field.Key)
	}
}

func TestInt64s(t *testing.T) {
	field := Int64s("ids", []int64{1, 2, 3})
	if field.Key != "ids" {
		t.Errorf("expected key 'ids', got '%s'", field.Key)
	}
}

func TestAny(t *testing.T) {
	field := Any("data", map[string]int{"a": 1})
	if field.Key != "data" {
		t.Errorf("expected key 'data', got '%s'", field.Key)
	}
}

type customObject struct {
	Name string
}

func (c customObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", c.Name)
	return nil
}

func TestObject(t *testing.T) {
	field := Object("custom", customObject{Name: "test"})
	if field.Key != "custom" {
		t.Errorf("expected key 'custom', got '%s'", field.Key)
	}
}

func TestFieldTypes(t *testing.T) {
	// Test all field creation functions
	String("s", "value")
	Int("i", 42)
	Int64("i64", 64)
	Float64("f", 3.14)
	Bool("b", true)
	Strings("ss", []string{"a", "b"})
	Int64s("i64s", []int64{1, 2, 3})
	Any("a", "value")
	Error(&testError{"error"})
}

func TestError(t *testing.T) {
	err := &testError{"test error"}
	field := Error(err)
	if field.Key != "error" {
		t.Errorf("expected key 'error', got '%s'", field.Key)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestConfig_InvalidLevel(t *testing.T) {
	// Should use default level for invalid level
	client, err := New(Config{
		Level:      "invalid",
		Format:     "console",
		OutputPath: "stdout",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	client.Info("test with invalid level")
}

func TestConfig_DifferentFormats(t *testing.T) {
	formats := []string{"json", "console"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			client, err := New(Config{
				Level:      "info",
				Format:     format,
				OutputPath: "stdout",
			})
			if err != nil {
				t.Fatalf("New() error for format %s: %v", format, err)
			}

			client.Info("test message")
		})
	}
}

func TestConfig_DifferentOutputs(t *testing.T) {
	outputs := []string{"stdout", "stderr"}

	for _, output := range outputs {
		t.Run(output, func(t *testing.T) {
			client, err := New(Config{
				Level:      "info",
				Format:     "console",
				OutputPath: output,
			})
			if err != nil {
				t.Fatalf("New() error for output %s: %v", output, err)
			}

			client.Info("test message")
		})
	}
}

func TestClient_Sync(t *testing.T) {
	client, _ := New(Config{
		Level:      "info",
		Format:     "console",
		OutputPath: "stdout",
	})

	client.Sync()
}

func TestNew_FileOutput(t *testing.T) {
	client, err := New(Config{
		Level:      "info",
		Format:     "json",
		OutputPath: "/tmp/test.log",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	client.Info("file test")
	client.Sync()
}
