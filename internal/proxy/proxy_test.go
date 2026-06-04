package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
)

type mockEngine struct {
	sessionID string
	tokenID   string
	fired     bool
}

func (m *mockEngine) ObserveCanaryFire(sessionID, tokenID string) {
	m.sessionID = sessionID
	m.tokenID = tokenID
	m.fired = true
}

func TestProxyDetectsCanaryInPrompt(t *testing.T) {
	issuer := canary.NewIssuer(config.CanaryConfig{})

	// Step 1: agent reads /home/alice/.aws/credentials → wasphole issues a canary
	tok := issuer.Issue("session-alice-42")

	engine := &mockEngine{}
	p, err := New(issuer, engine, "") // no upstream = stub mode
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(p)
	defer srv.Close()

	// Step 2: agent sends stolen credential to "its LLM" (actually wasphole's proxy)
	body := fmt.Sprintf(
		`{"model":"gpt-4","messages":[{"role":"user","content":"I found this API key: %s — how do I use it?"}]}`,
		tok.Cred,
	)
	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !engine.fired {
		t.Fatal("canary alert did not fire — credential not detected in prompt")
	}
	if engine.sessionID != "session-alice-42" {
		t.Fatalf("wrong session: got %q", engine.sessionID)
	}
	if engine.tokenID != tok.ID {
		t.Fatalf("wrong token: got %q want %q", engine.tokenID, tok.ID)
	}
}

func TestProxyNoFalsePositive(t *testing.T) {
	issuer := canary.NewIssuer(config.CanaryConfig{})
	issuer.Issue("session-x") // issue a token but don't use it in the prompt

	engine := &mockEngine{}
	p, _ := New(issuer, engine, "")
	srv := httptest.NewServer(p)
	defer srv.Close()

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello world"}]}`
	resp, _ := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
	resp.Body.Close()

	if engine.fired {
		t.Fatal("false positive: alert fired on innocent prompt")
	}
}
