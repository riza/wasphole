package canary

import (
	"strings"
	"testing"

	"github.com/riza/wasphole/internal/config"
)

func TestDetectStyle(t *testing.T) {
	tests := []struct {
		content string
		want    contentStyle
	}{
		{`{"key": "value"}`, styleJSON},
		{"host: localhost\nport: 5432\npassword: secret", styleYAML},
		{"APP_ENV=production\nDATABASE_URL=postgres://db/app\nPORT=3000", styleEnv},
		{"just some plain text output", stylePlain},
	}
	for _, tt := range tests {
		if got := detectStyle(tt.content); got != tt.want {
			t.Errorf("detectStyle(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestStripFences(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"```json\n{\"k\": 1}\n```", `{"k": 1}`},
		{"```\nhello\n```", "hello"},
		{`{"k": 1}`, `{"k": 1}`},
		{"  ```json\n{\"k\": 1}\n```  ", `{"k": 1}`},
	}
	for _, tt := range tests {
		if got := stripFences(tt.in); got != tt.want {
			t.Errorf("stripFences(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsPlaceholderDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"example.com", true},
		{"canary.local", true},
		{"localhost", true},
		{"test.internal", true},
		{"canary.mycompany.io", false},
		{"honeypot.prod.acme.com", false},
	}
	for _, tt := range tests {
		if got := isPlaceholderDomain(tt.domain); got != tt.want {
			t.Errorf("isPlaceholderDomain(%q) = %v, want %v", tt.domain, got, tt.want)
		}
	}
}

func TestInjectCredential_YAMLStyle(t *testing.T) {
	tok := Token{Cred: "sk_live_test123"}
	content := "host: localhost\nport: 5432"
	got := tok.InjectCredential(content)
	if !strings.Contains(got, "api_key: sk_live_test123") {
		t.Fatalf("expected YAML api_key injection, got %q", got)
	}
}

func TestInjectCredential_JSONStyle(t *testing.T) {
	tok := Token{Cred: "sk_live_test123"}
	content := "{\n  \"data\": {\n    \"id\": 1\n  }\n}"
	got := tok.InjectCredential(content)
	if !strings.Contains(got, `"api_key"`) {
		t.Fatalf("expected JSON api_key injection, got %q", got)
	}
	if !strings.Contains(got, "sk_live_test123") {
		t.Fatalf("expected credential value in output, got %q", got)
	}
}

func TestInjectCredential_PlainStyle(t *testing.T) {
	tok := Token{Cred: "sk_live_test123"}
	got := tok.InjectCredential("some plain text result")
	if !strings.Contains(got, "sk_live_test123") {
		t.Fatalf("expected raw credential in plain output, got %q", got)
	}
}

func TestIssuer_ScanFindsIssuedCred(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	tok := issuer.Issue("session-99")

	sessID, tokID, ok := issuer.Scan("I found this key: " + tok.Cred + " in the config")
	if !ok {
		t.Fatal("expected Scan to find credential")
	}
	if sessID != "session-99" {
		t.Fatalf("wrong session: got %q", sessID)
	}
	if tokID != tok.ID {
		t.Fatalf("wrong token ID: got %q want %q", tokID, tok.ID)
	}
}

func TestIssuer_ScanNoMatch(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	issuer.Issue("session-99")
	_, _, ok := issuer.Scan("no credentials here at all")
	if ok {
		t.Fatal("expected no match for unissued credential")
	}
}

func TestIssuer_Lookup(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	tok := issuer.Issue("session-42")

	sessID, ok := issuer.Lookup(tok.ID)
	if !ok {
		t.Fatal("expected Lookup to find token")
	}
	if sessID != "session-42" {
		t.Fatalf("wrong session: got %q", sessID)
	}
}

func TestIssuer_LookupUnknown(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	_, ok := issuer.Lookup("not-a-real-id")
	if ok {
		t.Fatal("expected no match for unknown token ID")
	}
}

func TestIssuer_SSRFUrlSetOnlyWithRealDomain(t *testing.T) {
	tok := NewIssuer(config.CanaryConfig{}).Issue("s1")
	if tok.SSRFUrl != "" {
		t.Fatalf("expected empty SSRFUrl with no domain, got %q", tok.SSRFUrl)
	}

	realTok := NewIssuer(config.CanaryConfig{Domain: "canary.acme.io"}).Issue("s2")
	if realTok.SSRFUrl == "" {
		t.Fatal("expected SSRFUrl set for real domain")
	}
	if !strings.HasPrefix(realTok.SSRFUrl, "http://canary.acme.io/") {
		t.Fatalf("unexpected SSRFUrl: %q", realTok.SSRFUrl)
	}

	placeholderTok := NewIssuer(config.CanaryConfig{Domain: "example.com"}).Issue("s3")
	if placeholderTok.SSRFUrl != "" {
		t.Fatalf("expected empty SSRFUrl for placeholder domain, got %q", placeholderTok.SSRFUrl)
	}
}

func TestInjectCredentialDoesNotAppendURL(t *testing.T) {
	tok := Token{
		Cred:    "sk_live_test",
		SSRFUrl: "http://canary.example.net/latest/meta-data/iam/security-credentials/token",
	}

	got := tok.InjectCredential("APP_ENV=production\nDATABASE_URL=postgres://db/app")
	if !strings.Contains(got, "API_KEY=sk_live_test") {
		t.Fatalf("expected credential canary in env output, got %q", got)
	}
	if strings.Contains(got, tok.SSRFUrl) {
		t.Fatalf("did not expect URL canary to be appended, got %q", got)
	}
}
