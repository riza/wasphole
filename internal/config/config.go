package config

// Config is the top-level configuration for wasphole.
type Config struct {
	Instance  InstanceConfig  `yaml:"instance"`
	AI        AIConfig        `yaml:"ai"`
	Transport TransportConfig `yaml:"transport"`
	Alerts    AlertsConfig    `yaml:"alerts"`
	Canary    CanaryConfig    `yaml:"canary"`
	Session   SessionConfig   `yaml:"session"`
	Proxy     ProxyConfig     `yaml:"proxy"`
}

// InstanceConfig controls the simulated environment (CFG-01).
type InstanceConfig struct {
	Mode     string `yaml:"mode"`     // "linux" or "windows" — required
	Industry string `yaml:"industry"` // e.g. "fintech", "healthcare", "saas" — optional
	Seed     string `yaml:"seed"`     // fixed seed for deterministic generation; random if empty
}

// AIConfig controls the AI provider used for identity and response generation (CFG-02).
type AIConfig struct {
	// APIType selects the wire protocol: "anthropic" or "openai".
	// Use "openai" for OpenAI-compatible providers (DeepSeek, MiniMax, GLM, etc.).
	APIType string `yaml:"api_type"` // "anthropic" or "openai" — required

	// BaseURL overrides the default API endpoint.
	// Leave empty to use the standard endpoint for the api_type.
	BaseURL string `yaml:"base_url"` // optional

	// APIKey is the provider API key.
	// If empty, falls back to ANTHROPIC_API_KEY (api_type=anthropic) or
	// OPENAI_API_KEY (api_type=openai) environment variable.
	APIKey string `yaml:"api_key"` // optional

	Model    string `yaml:"model"`     // model ID — required
	CacheDir string `yaml:"cache_dir"` // disk cache directory; defaults to "./cache"
}

// TransportConfig controls how the MCP server listens (CFG-03).
type TransportConfig struct {
	Type string    `yaml:"type"` // "stdio" or "http" — required
	Port int       `yaml:"port"` // TCP port when type=http; defaults to 8080
	TLS  TLSConfig `yaml:"tls"`
}

// TLSConfig enables HTTPS for the HTTP transport.
// Set either (CertFile + KeyFile) for a custom cert, or ACMEDomain for
// automatic Let's Encrypt certificate provisioning.
type TLSConfig struct {
	CertFile     string `yaml:"cert_file"`     // path to PEM certificate file
	KeyFile      string `yaml:"key_file"`      // path to PEM private key file
	ACMEDomain   string `yaml:"acme_domain"`   // domain for automatic Let's Encrypt cert
	ACMECacheDir string `yaml:"acme_cache_dir"` // directory to store ACME certs; defaults to ./acme-cache
}

// AlertsConfig holds one or more alert sinks (CFG-04).
type AlertsConfig struct {
	Sinks []AlertSink `yaml:"sinks"`
}

// AlertSink is a single alert output target.
type AlertSink struct {
	Type          string `yaml:"type"`           // "stdout", "webhook", or "siem"
	URL           string `yaml:"url"`            // webhook endpoint URL (type=webhook only)
	Format        string `yaml:"format"`         // "cef" or "json" (type=siem only)
	Path          string `yaml:"path"`           // output file path (type=siem only; empty = stderr)
	WebhookSecret string `yaml:"webhook_secret"` // base64 HMAC signing secret (type=webhook only)
	Enabled       bool   `yaml:"enabled"`        // defaults to true if omitted
}

// CanaryConfig controls canary token delivery (CFG-05).
type CanaryConfig struct {
	Domain       string `yaml:"domain"`        // DNS canary base domain, e.g. "canary.example.com"
	HTTPCallback string `yaml:"http_callback"` // URL to POST when a canary fires
	ListenAddr   string `yaml:"listen_addr"`   // address for the canary HTTP listener, e.g. ":9090"
}

// SessionConfig controls session recording (SR-01, SR-06).
type SessionConfig struct {
	LogDir string `yaml:"log_dir"` // directory for JSONL session logs; defaults to "./sessions"
}

// ProxyConfig enables the OpenAI-compatible canary-scanning proxy.
type ProxyConfig struct {
	ListenAddr  string `yaml:"listen_addr"`  // e.g. ":8081"; empty disables the proxy
	UpstreamURL string `yaml:"upstream_url"` // LLM API to forward to, e.g. "https://api.openai.com"
}
