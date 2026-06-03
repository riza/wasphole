package canary

import (
	"strings"
	"testing"
)

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
