package config

// Config is the top-level configuration for wasphole.
type Config struct {
	Instance  InstanceConfig  `yaml:"instance"`
	AI        AIConfig        `yaml:"ai"`
	Transport TransportConfig `yaml:"transport"`
	Alerts    AlertsConfig    `yaml:"alerts"`
	Canary    CanaryConfig    `yaml:"canary"`
}

// InstanceConfig controls the simulated environment (CFG-01).
type InstanceConfig struct {
	Mode     string `yaml:"mode"`     // "linux" or "windows" — required
	Industry string `yaml:"industry"` // e.g. "fintech", "healthcare", "saas" — optional
	Seed     string `yaml:"seed"`     // fixed seed for deterministic generation; random if empty
}

// AIConfig controls the AI provider used for identity and response generation (CFG-02).
type AIConfig struct {
	Provider string `yaml:"provider"` // "anthropic" — required
	Model    string `yaml:"model"`    // e.g. "claude-sonnet-4-6" — required
	CacheDir string `yaml:"cache_dir"` // disk cache directory; defaults to "./cache"
}

// TransportConfig controls how the MCP server listens (CFG-03).
type TransportConfig struct {
	Type string `yaml:"type"` // "stdio" or "http" — required
	Port int    `yaml:"port"` // TCP port when type=http; defaults to 8080
}

// AlertsConfig holds one or more alert sinks (CFG-04).
type AlertsConfig struct {
	Sinks []AlertSink `yaml:"sinks"`
}

// AlertSink is a single alert output target.
type AlertSink struct {
	Type    string `yaml:"type"`    // "stdout", "webhook", or "siem"
	URL     string `yaml:"url"`     // webhook endpoint URL (type=webhook only)
	Format  string `yaml:"format"`  // "cef" or "json" (type=siem only)
	Enabled bool   `yaml:"enabled"` // defaults to true if omitted
}

// CanaryConfig controls canary token delivery (CFG-05).
type CanaryConfig struct {
	Domain       string `yaml:"domain"`        // DNS canary base domain, e.g. "canary.example.com"
	HTTPCallback string `yaml:"http_callback"` // URL to POST when a canary fires
}
