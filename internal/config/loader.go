package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file at path, validates required fields, and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.AI.CacheDir == "" {
		cfg.AI.CacheDir = "./cache"
	}
	if cfg.Session.LogDir == "" {
		cfg.Session.LogDir = "./sessions"
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "claude-sonnet-4-6"
	}
	if cfg.Transport.Port == 0 {
		cfg.Transport.Port = 8080
	}
}

func validate(cfg *Config) error {
	switch cfg.Instance.Mode {
	case "linux", "windows":
	default:
		return fmt.Errorf("config: instance.mode must be \"linux\" or \"windows\", got %q", cfg.Instance.Mode)
	}

	switch cfg.Transport.Type {
	case "stdio", "http":
	default:
		return fmt.Errorf("config: transport.type must be \"stdio\" or \"http\", got %q", cfg.Transport.Type)
	}

	switch cfg.AI.APIType {
	case "anthropic", "openai":
	default:
		return fmt.Errorf("config: ai.api_type must be \"anthropic\" or \"openai\", got %q", cfg.AI.APIType)
	}

	return nil
}
