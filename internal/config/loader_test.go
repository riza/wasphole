package config

import (
	"os"
	"testing"
)

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoadValid(t *testing.T) {
	path := writeConfig(t, `
instance:
  mode: linux
transport:
  type: http
ai:
  api_type: anthropic
  model: claude-sonnet-4-6
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Instance.Mode != "linux" {
		t.Errorf("Mode = %q, want linux", cfg.Instance.Mode)
	}
	if cfg.AI.CacheDir != "./cache" {
		t.Errorf("CacheDir = %q, want ./cache (default)", cfg.AI.CacheDir)
	}
	if cfg.Transport.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Transport.Port)
	}
	if cfg.Session.LogDir != "./sessions" {
		t.Errorf("LogDir = %q, want ./sessions (default)", cfg.Session.LogDir)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeConfig(t, "{{ invalid yaml }")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestApplyDefaults_Empty(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	if cfg.AI.CacheDir != "./cache" {
		t.Errorf("CacheDir = %q, want ./cache", cfg.AI.CacheDir)
	}
	if cfg.Session.LogDir != "./sessions" {
		t.Errorf("LogDir = %q, want ./sessions", cfg.Session.LogDir)
	}
	if cfg.AI.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", cfg.AI.Model)
	}
	if cfg.Transport.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Transport.Port)
	}
}

func TestApplyDefaults_DoesNotOverrideExisting(t *testing.T) {
	cfg := &Config{
		AI:        AIConfig{CacheDir: "./custom-cache", Model: "claude-opus-4-8"},
		Session:   SessionConfig{LogDir: "./my-sessions"},
		Transport: TransportConfig{Port: 9090},
	}
	applyDefaults(cfg)

	if cfg.AI.CacheDir != "./custom-cache" {
		t.Errorf("CacheDir overridden: got %q", cfg.AI.CacheDir)
	}
	if cfg.AI.Model != "claude-opus-4-8" {
		t.Errorf("Model overridden: got %q", cfg.AI.Model)
	}
	if cfg.Transport.Port != 9090 {
		t.Errorf("Port overridden: got %d", cfg.Transport.Port)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	cfg := &Config{
		Instance:  InstanceConfig{Mode: "freebsd"},
		Transport: TransportConfig{Type: "http"},
		AI:        AIConfig{APIType: "anthropic"},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for invalid mode freebsd")
	}
}

func TestValidate_InvalidTransport(t *testing.T) {
	cfg := &Config{
		Instance:  InstanceConfig{Mode: "linux"},
		Transport: TransportConfig{Type: "ftp"},
		AI:        AIConfig{APIType: "anthropic"},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for invalid transport type ftp")
	}
}

func TestValidate_InvalidAPIType(t *testing.T) {
	cfg := &Config{
		Instance:  InstanceConfig{Mode: "linux"},
		Transport: TransportConfig{Type: "http"},
		AI:        AIConfig{APIType: "gemini"},
	}
	if err := validate(cfg); err == nil {
		t.Fatal("expected error for invalid api_type gemini")
	}
}

func TestValidate_AllValidCombinations(t *testing.T) {
	combos := []struct{ mode, ttype, apiType string }{
		{"linux", "stdio", "anthropic"},
		{"linux", "http", "openai"},
		{"windows", "stdio", "anthropic"},
		{"windows", "http", "openai"},
	}
	for _, c := range combos {
		cfg := &Config{
			Instance:  InstanceConfig{Mode: c.mode},
			Transport: TransportConfig{Type: c.ttype},
			AI:        AIConfig{APIType: c.apiType},
		}
		if err := validate(cfg); err != nil {
			t.Errorf("validate(%v) unexpected error: %v", c, err)
		}
	}
}
