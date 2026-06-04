package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// helper: minimal valid setupConfig
func baseConfig() setupConfig {
	return setupConfig{
		Mode:          "linux",
		Provider:      "anthropic",
		Model:         "claude-sonnet-4-6",
		CacheDir:      "./cache",
		Transport:     "http",
		Port:          8080,
		TLSMode:       "none",
		SIEMFormat:    "none",
		SessionLogDir: "./sessions",
	}
}

// ── buildConfig ───────────────────────────────────────────────────────────────

func TestBuildConfig_ContainsRequiredFields(t *testing.T) {
	c := baseConfig()
	c.Industry = "fintech"
	out := buildConfig(c)

	for _, want := range []string{
		"mode: linux",
		"industry: fintech",
		"api_type: anthropic",
		"model: claude-sonnet-4-6",
		"type: http",
		"port: 8080",
		"type: stdout",
		"log_dir: ./sessions",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("buildConfig missing %q", want)
		}
	}
}

func TestBuildConfig_OmitsIndustryWhenBlank(t *testing.T) {
	out := buildConfig(baseConfig())
	if strings.Contains(out, "industry:") {
		t.Error("expected industry to be omitted when blank")
	}
}

func TestBuildConfig_OmitsPortForStdio(t *testing.T) {
	c := baseConfig()
	c.Transport = "stdio"
	c.Port = 0
	if strings.Contains(buildConfig(c), "  port:") {
		t.Error("expected port to be omitted for stdio transport")
	}
}

func TestBuildConfig_WebhookIncluded(t *testing.T) {
	c := baseConfig()
	c.WebhookURL    = "https://hooks.slack.com/xyz"
	c.WebhookSecret = "base64secret"
	out := buildConfig(c)
	if !strings.Contains(out, "url: https://hooks.slack.com/xyz") {
		t.Error("expected webhook URL")
	}
	if !strings.Contains(out, "webhook_secret") {
		t.Error("expected webhook_secret")
	}
}

func TestBuildConfig_WebhookOmittedWhenBlank(t *testing.T) {
	if strings.Contains(buildConfig(baseConfig()), "type: webhook") {
		t.Error("expected webhook sink omitted when URL blank")
	}
}

func TestBuildConfig_SIEMIncluded(t *testing.T) {
	c := baseConfig()
	c.SIEMFormat = "cef"
	c.SIEMPath   = "/var/log/wasphole/siem.log"
	out := buildConfig(c)
	if !strings.Contains(out, "format: cef") {
		t.Error("expected SIEM format")
	}
	if !strings.Contains(out, `path: "/var/log/wasphole/siem.log"`) {
		t.Error("expected SIEM path")
	}
}

func TestBuildConfig_TLSCert(t *testing.T) {
	c := baseConfig()
	c.TLSMode = "cert + key"
	c.TLSCert = "/etc/tls/cert.pem"
	c.TLSKey  = "/etc/tls/key.pem"
	out := buildConfig(c)
	if !strings.Contains(out, `cert_file: "/etc/tls/cert.pem"`) {
		t.Error("expected cert_file")
	}
}

func TestBuildConfig_TLSAcme(t *testing.T) {
	c := baseConfig()
	c.TLSMode     = "Let's Encrypt"
	c.ACMEDomain  = "honeypot.example.com"
	out := buildConfig(c)
	if !strings.Contains(out, `acme_domain: "honeypot.example.com"`) {
		t.Error("expected acme_domain")
	}
}

func TestBuildConfig_CanaryFields(t *testing.T) {
	c := baseConfig()
	c.CanaryDomain   = "canary.example.com"
	c.CanaryCallback = "https://hooks.example.com/canary"
	c.CanaryAddr     = ":9090"
	out := buildConfig(c)
	if !strings.Contains(out, `domain: "canary.example.com"`) {
		t.Error("expected canary domain")
	}
	if !strings.Contains(out, `listen_addr: ":9090"`) {
		t.Error("expected canary listen_addr")
	}
}

func TestBuildConfig_ProxyFields(t *testing.T) {
	c := baseConfig()
	c.ProxyAddr     = ":8081"
	c.ProxyUpstream = "https://api.openai.com"
	out := buildConfig(c)
	if !strings.Contains(out, `listen_addr: ":8081"`) {
		t.Error("expected proxy listen_addr")
	}
	if !strings.Contains(out, `upstream_url: "https://api.openai.com"`) {
		t.Error("expected proxy upstream_url")
	}
}

func TestBuildConfig_BaseURLAndAPIKey(t *testing.T) {
	c := baseConfig()
	c.Provider = "openai"
	c.BaseURL  = "https://api.deepseek.com/v1"
	c.APIKey   = "sk-test-key"
	c.Model    = "deepseek-chat"
	out := buildConfig(c)
	if !strings.Contains(out, `base_url: "https://api.deepseek.com/v1"`) {
		t.Error("expected base_url")
	}
	if !strings.Contains(out, `api_key: "sk-test-key"`) {
		t.Error("expected api_key")
	}
}

func TestBuildConfig_SeedIncluded(t *testing.T) {
	c := baseConfig()
	c.Seed = "my-deterministic-seed"
	if !strings.Contains(buildConfig(c), "seed: my-deterministic-seed") {
		t.Error("expected seed in config")
	}
}

func TestBuildConfig_SeedOmittedWhenBlank(t *testing.T) {
	if strings.Contains(buildConfig(baseConfig()), "seed:") {
		t.Error("expected seed omitted when blank")
	}
}

func TestBuildConfig_ValidYAML(t *testing.T) {
	out := buildConfig(baseConfig())
	if strings.Contains(out, "\t") {
		t.Error("config must not contain tab characters")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("config must end with newline")
	}
}

// ── pickOne ───────────────────────────────────────────────────────────────────

func TestPickOne_ByNumber(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("2\n"))
	if got := pickOne(r, "OS?", []string{"linux", "windows"}); got != "windows" {
		t.Errorf("got %q, want windows", got)
	}
}

func TestPickOne_ByName(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("linux\n"))
	if got := pickOne(r, "OS?", []string{"linux", "windows"}); got != "linux" {
		t.Errorf("got %q, want linux", got)
	}
}

func TestPickOne_CaseInsensitive(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("WINDOWS\n"))
	if got := pickOne(r, "OS?", []string{"linux", "windows"}); got != "windows" {
		t.Errorf("got %q, want windows", got)
	}
}

func TestPickOne_RetriesOnInvalid(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("bad\n1\n"))
	if got := pickOne(r, "OS?", []string{"linux", "windows"}); got != "linux" {
		t.Errorf("got %q, want linux", got)
	}
}

// ── askFree ───────────────────────────────────────────────────────────────────

func TestAskFree_ReturnsInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("my-value\n"))
	if got := askFree(r, "Label?", "default", ""); got != "my-value" {
		t.Errorf("got %q, want my-value", got)
	}
}

func TestAskFree_ReturnsDefaultOnBlank(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := askFree(r, "Label?", "default-val", ""); got != "default-val" {
		t.Errorf("got %q, want default-val", got)
	}
}

func TestAskFree_EmptyDefaultAndBlankInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := askFree(r, "Label?", "", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// ── runSetup end-to-end ───────────────────────────────────────────────────────

func TestRunSetup_WritesConfigFile(t *testing.T) {
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Answers for all wizard questions in order:
	// OS=1(linux), industry=fintech, seed=blank, provider=1(anthropic),
	// baseURL=blank, apiKey=blank, model=blank(default), cacheDir=blank(default),
	// transport=1(http), port=blank(8080), tls=1(none),
	// webhook=blank, siem=1(none),
	// canaryDomain=blank, canaryCallback=blank, canaryAddr=blank,
	// proxyAddr=blank,
	// sessionLogDir=blank(default)
	go func() {
		w.WriteString("1\nfintech\n\n1\n\n\n\n\n1\n\n1\n\n1\n\n\n\n\n\n")
		w.Close()
	}()

	origWD, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer func() {
		os.Stdin = origStdin
		os.Chdir(origWD)
	}()

	runSetup()

	data, err := os.ReadFile(dir + "/config.yaml")
	if err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
	out := string(data)
	for _, want := range []string{"mode: linux", "industry: fintech", "api_type: anthropic"} {
		if !strings.Contains(out, want) {
			t.Errorf("config missing %q\n%s", want, out)
		}
	}
}
