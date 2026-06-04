package canary

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/riza/wasphole/internal/config"
)

type mockAlertEngine struct {
	fired     bool
	sessionID string
	tokenID   string
}

func (m *mockAlertEngine) ObserveCanaryFire(sessionID, tokenID string) {
	m.fired = true
	m.sessionID = sessionID
	m.tokenID = tokenID
}

func TestListenerUnknownToken(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	eng := &mockAlertEngine{}
	l := NewListener(issuer, "", eng)
	srv := httptest.NewServer(l)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/c/unknownid")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if eng.fired {
		t.Fatal("engine should not fire for unknown token")
	}
}

func TestListenerEmptyPath(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	l := NewListener(issuer, "", nil)
	srv := httptest.NewServer(l)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for empty path, got %d", resp.StatusCode)
	}
}

func TestListenerKnownToken(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	tok := issuer.Issue("session-abc")
	eng := &mockAlertEngine{}
	l := NewListener(issuer, "", eng)
	srv := httptest.NewServer(l)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/c/" + tok.ID)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !eng.fired {
		t.Fatal("engine should fire for known token")
	}
	if eng.sessionID != "session-abc" {
		t.Fatalf("wrong session: got %q", eng.sessionID)
	}
	if eng.tokenID != tok.ID {
		t.Fatalf("wrong token ID: got %q want %q", eng.tokenID, tok.ID)
	}
}

func TestListenerIMDSPath(t *testing.T) {
	issuer := NewIssuer(config.CanaryConfig{})
	tok := issuer.Issue("session-imds")
	eng := &mockAlertEngine{}
	l := NewListener(issuer, "", eng)
	srv := httptest.NewServer(l)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/latest/meta-data/iam/security-credentials/" + tok.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	if !strings.Contains(string(body), `"Code": "Success"`) {
		t.Fatalf("expected IMDS response body, got %q", string(body))
	}
	if !eng.fired {
		t.Fatal("engine should fire for IMDS path")
	}
}

func TestLastPathSegment(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/a/b/c", "c"},
		{"/c/", "c"},
		{"/", ""},
		{"", ""},
		{"/single", "single"},
		{"//double//slashes//", "slashes"},
	}
	for _, tt := range tests {
		if got := lastPathSegment(tt.path); got != tt.want {
			t.Errorf("lastPathSegment(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsIMDSPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/latest/meta-data/instance-id", true},
		{"/computeMetadata/v1/project-id", true},
		{"/metadata/v1/id", true},
		{"/api/v1/users", false},
		{"/c/tokenid", false},
	}
	for _, tt := range tests {
		if got := isIMDSPath(tt.path); got != tt.want {
			t.Errorf("isIMDSPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
