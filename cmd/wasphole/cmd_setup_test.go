package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// ── buildConfig ───────────────────────────────────────────────────────────────

func TestBuildConfig_ContainsRequiredFields(t *testing.T) {
	out := buildConfig("linux", "fintech", "anthropic", "", "", "claude-sonnet-4-6", "http", 8080, "")

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
	out := buildConfig("linux", "", "anthropic", "", "", "claude-sonnet-4-6", "http", 8080, "")
	if strings.Contains(out, "industry:") {
		t.Error("expected industry to be omitted when blank")
	}
}

func TestBuildConfig_OmitsPortForStdio(t *testing.T) {
	out := buildConfig("linux", "", "anthropic", "", "", "claude-sonnet-4-6", "stdio", 0, "")
	if strings.Contains(out, "  port:") {
		t.Error("expected port to be omitted for stdio transport")
	}
}

func TestBuildConfig_IncludesWebhookWhenSet(t *testing.T) {
	out := buildConfig("windows", "", "openai", "", "", "gpt-4o", "http", 8080, "https://hooks.slack.com/xyz")
	if !strings.Contains(out, "url: https://hooks.slack.com/xyz") {
		t.Error("expected webhook URL in config")
	}
	if !strings.Contains(out, "type: webhook") {
		t.Error("expected webhook sink type in config")
	}
}

func TestBuildConfig_OmitsWebhookWhenBlank(t *testing.T) {
	out := buildConfig("linux", "", "anthropic", "", "", "claude-sonnet-4-6", "http", 8080, "")
	if strings.Contains(out, "type: webhook") {
		t.Error("expected webhook sink to be omitted when URL is blank")
	}
}

func TestBuildConfig_BaseURLAndAPIKey(t *testing.T) {
	out := buildConfig("linux", "", "openai", "https://api.deepseek.com/v1", "sk-test-key", "deepseek-chat", "http", 8080, "")
	if !strings.Contains(out, `base_url: "https://api.deepseek.com/v1"`) {
		t.Error("expected base_url in config")
	}
	if !strings.Contains(out, `api_key: "sk-test-key"`) {
		t.Error("expected api_key in config")
	}
}

func TestBuildConfig_ValidYAML(t *testing.T) {
	out := buildConfig("windows", "healthcare", "openai", "", "", "gpt-4o", "http", 9090, "https://hooks.example.com/alert")
	// Must contain no tabs (YAML indent must be spaces)
	if strings.Contains(out, "\t") {
		t.Error("config must not contain tab characters")
	}
	// Must have a newline at end
	if !strings.HasSuffix(out, "\n") {
		t.Error("config must end with newline")
	}
}

// ── pickOne ───────────────────────────────────────────────────────────────────

func TestPickOne_ByNumber(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("2\n"))
	got := pickOne(r, "OS?", []string{"linux", "windows"})
	if got != "windows" {
		t.Errorf("pickOne by number: got %q, want windows", got)
	}
}

func TestPickOne_ByName(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("linux\n"))
	got := pickOne(r, "OS?", []string{"linux", "windows"})
	if got != "linux" {
		t.Errorf("pickOne by name: got %q, want linux", got)
	}
}

func TestPickOne_CaseInsensitive(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("WINDOWS\n"))
	got := pickOne(r, "OS?", []string{"linux", "windows"})
	if got != "windows" {
		t.Errorf("pickOne case-insensitive: got %q, want windows", got)
	}
}

func TestPickOne_RetriesOnInvalid(t *testing.T) {
	// "bad" is invalid, then "1" is valid
	r := bufio.NewReader(strings.NewReader("bad\n1\n"))
	got := pickOne(r, "OS?", []string{"linux", "windows"})
	if got != "linux" {
		t.Errorf("pickOne retry: got %q, want linux", got)
	}
}

// ── askFree ───────────────────────────────────────────────────────────────────

func TestAskFree_ReturnsInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("my-value\n"))
	got := askFree(r, "Label?", "default", "")
	if got != "my-value" {
		t.Errorf("askFree: got %q, want my-value", got)
	}
}

func TestAskFree_ReturnsDefaultOnBlank(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := askFree(r, "Label?", "default-val", "")
	if got != "default-val" {
		t.Errorf("askFree default: got %q, want default-val", got)
	}
}

func TestAskFree_EmptyDefaultAndBlankInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := askFree(r, "Label?", "", "")
	if got != "" {
		t.Errorf("askFree empty: got %q, want empty", got)
	}
}

// ── runSetup end-to-end ───────────────────────────────────────────────────────

func TestRunSetup_WritesConfigFile(t *testing.T) {
	dir := t.TempDir()
	outPath := dir + "/config.yaml"

	// Patch stdin and output path for the test
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r

	// Write answers: linux, fintech, anthropic, (blank baseURL), (blank key),
	// (blank model=default), http, (blank port=8080), (blank webhook)
	go func() {
		w.WriteString("1\nfintech\n1\n\n\n\n1\n\n\n")
		w.Close()
	}()

	// Temporarily redirect output path
	origWD, _ := os.Getwd()
	os.Chdir(dir)
	defer func() {
		os.Stdin = origStdin
		os.Chdir(origWD)
	}()

	runSetup()

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "mode: linux") {
		t.Errorf("config missing mode: linux\n%s", data)
	}
	if !strings.Contains(string(data), "api_type: anthropic") {
		t.Errorf("config missing api_type\n%s", data)
	}
}
