package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	clrReset  = "\033[0m"
	clrBold   = "\033[1m"
	clrFaint  = "\033[2m"
	clrAccent = "\033[33m"
)

// setupConfig holds every value collected by the wizard.
type setupConfig struct {
	// instance
	Mode     string
	Industry string
	Seed     string

	// ai
	Provider  string
	BaseURL   string
	APIKey    string
	Model     string
	CacheDir  string

	// transport
	Transport    string
	Port         int
	TLSMode      string // "none" | "cert" | "acme"
	TLSCert      string
	TLSKey       string
	ACMEDomain   string
	ACMECacheDir string

	// alerts
	WebhookURL    string
	WebhookSecret string
	SIEMFormat    string // "none" | "cef" | "json"
	SIEMPath      string

	// canary
	CanaryDomain   string
	CanaryCallback string
	CanaryAddr     string

	// proxy
	ProxyAddr   string
	ProxyUpstream string

	// session
	SessionLogDir string
}

func runSetup() {
	r := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println(clrBold + "  wasphole setup" + clrReset)
	fmt.Println("  " + strings.Repeat("─", 32))

	var c setupConfig

	// ── Instance ──────────────────────────────────────────────────
	section("Instance")
	c.Mode     = pickOne(r, "Simulated OS?", []string{"linux", "windows"})
	c.Industry = askFree(r, "Industry?", "", "fintech, healthcare, saas … leave blank to skip")
	c.Seed     = askFree(r, "Seed?", "", "fixed seed for deterministic generation — leave blank for random")

	// ── AI ────────────────────────────────────────────────────────
	section("AI")
	c.Provider = pickOne(r, "AI provider?", []string{"anthropic", "openai"})
	c.BaseURL  = askFree(r, "Base URL?", "", "leave blank for default endpoint")

	envVar       := "ANTHROPIC_API_KEY"
	defaultModel := "claude-sonnet-4-6"
	if c.Provider == "openai" {
		envVar       = "OPENAI_API_KEY"
		defaultModel = "gpt-4o"
	}
	c.APIKey   = askFree(r, "API key?", "", "or set "+envVar+" env var — leave blank to skip")
	c.Model    = askFree(r, "Model?", defaultModel, "")
	c.CacheDir = askFree(r, "Cache dir?", "./cache", "")

	// ── Transport ─────────────────────────────────────────────────
	section("Transport")
	c.Transport = pickOne(r, "Transport?", []string{"http", "stdio"})

	if c.Transport == "http" {
		raw := askFree(r, "Port?", "8080", "")
		if p, err := strconv.Atoi(raw); err == nil && p > 0 {
			c.Port = p
		} else {
			c.Port = 8080
		}

		c.TLSMode = pickOne(r, "TLS?", []string{"none", "cert + key", "Let's Encrypt"})
		switch c.TLSMode {
		case "cert + key":
			c.TLSCert = askFree(r, "Cert file?", "", "e.g. /etc/wasphole/tls/cert.pem")
			c.TLSKey  = askFree(r, "Key file?", "", "e.g. /etc/wasphole/tls/key.pem")
		case "Let's Encrypt":
			c.ACMEDomain   = askFree(r, "ACME domain?", "", "e.g. honeypot.example.com")
			c.ACMECacheDir = askFree(r, "ACME cache dir?", "./acme-cache", "")
		}
	}

	// ── Alerts ────────────────────────────────────────────────────
	section("Alerts")
	c.WebhookURL = askFree(r, "Webhook URL?", "", "leave blank to skip")
	if c.WebhookURL != "" {
		c.WebhookSecret = askFree(r, "Webhook secret?", "", "base64 HMAC signing key — leave blank to skip")
	}
	c.SIEMFormat = pickOne(r, "SIEM output?", []string{"none", "cef", "json"})
	if c.SIEMFormat != "none" {
		c.SIEMPath = askFree(r, "SIEM path?", "", "output file — leave blank to write to stderr")
	}

	// ── Canary ────────────────────────────────────────────────────
	section("Canary")
	c.CanaryDomain   = askFree(r, "Canary domain?", "", "e.g. canary.example.com — leave blank to skip")
	c.CanaryCallback = askFree(r, "HTTP callback URL?", "", "POST when a canary fires — leave blank to skip")
	c.CanaryAddr     = askFree(r, "Canary listener addr?", "", "e.g. :9090 — leave blank to skip")

	// ── Proxy ─────────────────────────────────────────────────────
	section("Proxy")
	c.ProxyAddr = askFree(r, "Proxy listen addr?", "", "OpenAI-compatible canary proxy — e.g. :8081 — leave blank to skip")
	if c.ProxyAddr != "" {
		c.ProxyUpstream = askFree(r, "Proxy upstream URL?", "", "real LLM API to forward to — e.g. https://api.openai.com")
	}

	// ── Session ───────────────────────────────────────────────────
	section("Session")
	c.SessionLogDir = askFree(r, "Session log dir?", "./sessions", "")

	// ── Write ─────────────────────────────────────────────────────
	outPath := "config.yaml"
	if _, err := os.Stat(outPath); err == nil {
		overwrite := askFree(r, "config.yaml already exists. Overwrite?", "no", "yes / no")
		if !strings.HasPrefix(strings.ToLower(overwrite), "y") {
			fmt.Println()
			fmt.Println(clrFaint + "  Aborted — config.yaml unchanged." + clrReset)
			fmt.Println()
			return
		}
	}

	if err := os.WriteFile(outPath, []byte(buildConfig(c)), 0o640); err != nil {
		fmt.Fprintf(os.Stderr, "  error writing config.yaml: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(clrAccent + "  ✓ config.yaml written" + clrReset)
	fmt.Println()
	fmt.Println(clrFaint + "  Next: wasphole server" + clrReset)
	fmt.Println()
}

func section(name string) {
	fmt.Println()
	fmt.Println(clrFaint + "  ── " + name + " " + strings.Repeat("─", max(0, 28-len(name))) + clrReset)
	fmt.Println()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Prompt helpers ────────────────────────────────────────────────────────────

func pickOne(r *bufio.Reader, label string, options []string) string {
	for {
		fmt.Println("  " + clrBold + label + clrReset)
		for i, o := range options {
			fmt.Printf("  [%d] %s\n", i+1, o)
		}
		fmt.Print(clrAccent + "  → " + clrReset)
		raw := readLine(r)

		if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= len(options) {
			fmt.Println()
			return options[n-1]
		}
		for _, o := range options {
			if strings.EqualFold(raw, o) {
				fmt.Println()
				return o
			}
		}
		fmt.Println(clrFaint + "  Please enter a number or one of: " + strings.Join(options, ", ") + clrReset)
	}
}

func askFree(r *bufio.Reader, label, def, hint string) string {
	display := "  " + clrBold + label + clrReset
	if def != "" {
		display += clrFaint + " [" + def + "]" + clrReset
	}
	if hint != "" {
		display += clrFaint + " (" + hint + ")" + clrReset
	}
	fmt.Println(display)
	fmt.Print(clrAccent + "  → " + clrReset)
	raw := readLine(r)
	fmt.Println()
	if raw == "" {
		return def
	}
	return raw
}

func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

// ── Config builder ────────────────────────────────────────────────────────────

func buildConfig(c setupConfig) string {
	var b strings.Builder

	w := func(s string) { b.WriteString(s) }
	nl := func() { b.WriteString("\n") }

	w("# generated by wasphole setup\n\n")

	// instance
	w("instance:\n")
	w("  mode: " + c.Mode + "\n")
	if c.Industry != "" {
		w("  industry: " + c.Industry + "\n")
	}
	if c.Seed != "" {
		w("  seed: " + c.Seed + "\n")
	}
	nl()

	// ai
	w("ai:\n")
	w("  api_type: " + c.Provider + "\n")
	w("  base_url: \"" + c.BaseURL + "\"\n")
	w("  api_key: \"" + c.APIKey + "\"\n")
	w("  model: " + c.Model + "\n")
	w("  cache_dir: " + c.CacheDir + "\n")
	nl()

	// transport
	w("transport:\n")
	w("  type: " + c.Transport + "\n")
	if c.Transport == "http" {
		w("  port: " + strconv.Itoa(c.Port) + "\n")
		w("  tls:\n")
		switch c.TLSMode {
		case "cert + key":
			w("    cert_file: \"" + c.TLSCert + "\"\n")
			w("    key_file: \"" + c.TLSKey + "\"\n")
		case "Let's Encrypt":
			w("    acme_domain: \"" + c.ACMEDomain + "\"\n")
			w("    acme_cache_dir: \"" + c.ACMECacheDir + "\"\n")
		default:
			w("    cert_file: \"\"\n")
			w("    key_file: \"\"\n")
		}
	}
	nl()

	// alerts
	w("alerts:\n")
	w("  sinks:\n")
	w("    - type: stdout\n")
	w("      enabled: true\n")
	if c.WebhookURL != "" {
		w("    - type: webhook\n")
		w("      url: " + c.WebhookURL + "\n")
		if c.WebhookSecret != "" {
			w("      webhook_secret: \"" + c.WebhookSecret + "\"\n")
		}
		w("      enabled: true\n")
	}
	if c.SIEMFormat != "none" {
		w("    - type: siem\n")
		w("      format: " + c.SIEMFormat + "\n")
		if c.SIEMPath != "" {
			w("      path: \"" + c.SIEMPath + "\"\n")
		}
		w("      enabled: true\n")
	}
	nl()

	// canary
	w("canary:\n")
	w("  domain: \"" + c.CanaryDomain + "\"\n")
	w("  http_callback: \"" + c.CanaryCallback + "\"\n")
	w("  listen_addr: \"" + c.CanaryAddr + "\"\n")
	nl()

	// proxy
	w("proxy:\n")
	w("  listen_addr: \"" + c.ProxyAddr + "\"\n")
	w("  upstream_url: \"" + c.ProxyUpstream + "\"\n")
	nl()

	// session
	w("session:\n")
	w("  log_dir: " + c.SessionLogDir + "\n")

	return b.String()
}
