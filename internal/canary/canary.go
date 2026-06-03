package canary

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/riza/wasphole/internal/config"
)

// Token is a set of canary artifacts issued for a response that naturally
// contains a secret-bearing value.
type Token struct {
	ID      string // pure random hex — map lookup key, embedded in URLs
	Cred    string // realistic API key (sk_live_xxx, pat_xxx, etc.)
	SSRFUrl string // AWS IMDS-style URL to detect SSRF attempts
	DNS     string // DNS subdomain canary
}

// InjectCredential embeds only the credential canary using a style that matches
// the content format. URL/DNS canaries are not inserted here; sprinkling callback
// URLs into arbitrary command output makes the honeypot easy to fingerprint.
func (t Token) InjectCredential(content string) string {
	// Strip markdown fences before style detection.
	content = stripFences(content)

	switch detectStyle(content) {
	case styleYAML:
		content += "\napi_key: " + t.Cred
	case styleEnv:
		content += "\nAPI_KEY=" + t.Cred
	case styleJSON:
		// Prefer injecting inside the first nested object (data, result, etc.)
		// so the credential doesn't float at the root level.
		if pos := strings.Index(content, "\n  }"); pos != -1 {
			content = content[:pos] + fmt.Sprintf(",\n    \"api_key\": %q", t.Cred) + content[pos:]
		} else if idx := strings.LastIndex(content, "}"); idx != -1 {
			content = content[:idx] + fmt.Sprintf(",\n  \"api_key\": %q\n", t.Cred) + content[idx:]
		}
	default:
		content += "\n" + t.Cred
	}
	return content
}

// stripFences removes markdown code fences (```json ... ```) from AI responses.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	for _, open := range []string{"```json\n", "```\n", "```json", "```"} {
		if strings.HasPrefix(s, open) {
			s = s[len(open):]
			break
		}
	}
	if idx := strings.LastIndex(s, "```"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

type contentStyle int

const (
	stylePlain contentStyle = iota
	styleYAML
	styleEnv
	styleJSON
)

// detectStyle samples lines to guess the content format.
func detectStyle(content string) contentStyle {
	if strings.HasPrefix(strings.TrimSpace(content), "{") {
		return styleJSON
	}
	yamlScore, envScore := 0, 0
	for _, line := range strings.SplitN(content, "\n", 20) {
		if strings.Contains(line, ": ") {
			yamlScore++
		}
		if idx := strings.Index(line, "="); idx > 0 && !strings.Contains(line[:idx], " ") {
			envScore++
		}
	}
	if yamlScore >= 2 {
		return styleYAML
	}
	if envScore >= 2 {
		return styleEnv
	}
	return stylePlain
}

// Issuer creates and resolves canary tokens.
type Issuer struct {
	cfg    config.CanaryConfig
	mu     sync.RWMutex
	tokens map[string]string // tokenID → sessionID
}

// NewIssuer constructs an Issuer from the canary config.
func NewIssuer(cfg config.CanaryConfig) *Issuer {
	return &Issuer{cfg: cfg, tokens: make(map[string]string)}
}

// Issue generates a new Token for the given session and records it for lookup.
func (i *Issuer) Issue(sessionID string) Token {
	id := mustHex(12) // 24-char random ID — no session info in it

	cred := credPrefix() + mustHex(20)

	var ssrfURL, dns string
	if i.cfg.Domain != "" && !isPlaceholderDomain(i.cfg.Domain) {
		// Looks like an AWS IMDS credentials endpoint — agents doing cloud recon will fetch it.
		ssrfURL = "http://" + i.cfg.Domain + "/latest/meta-data/iam/security-credentials/" + id
		dns = id + "." + i.cfg.Domain
	}

	tok := Token{ID: id, Cred: cred, SSRFUrl: ssrfURL, DNS: dns}

	i.mu.Lock()
	i.tokens[id] = sessionID
	i.mu.Unlock()

	return tok
}

// Lookup resolves a token ID to the originating session ID.
func (i *Issuer) Lookup(id string) (string, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	sess, ok := i.tokens[id]
	return sess, ok
}

// isPlaceholderDomain returns true for domains that are clearly not real.
func isPlaceholderDomain(domain string) bool {
	for _, fake := range []string{"example.com", "example.org", "localhost", "test.", "canary.local"} {
		if strings.Contains(domain, fake) {
			return true
		}
	}
	return false
}

// credPrefix returns a realistic API key prefix varied per token.
func credPrefix() string {
	prefixes := []string{"sk_live_", "sk-proj-", "pat_", "api_", "token_", "ghp_"}
	b := mustBytes(1)
	return prefixes[b[0]%byte(len(prefixes))]
}

func mustHex(n int) string {
	return hex.EncodeToString(mustBytes(n))
}

func mustBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("canary: crypto/rand failed: " + err.Error())
	}
	return b
}
