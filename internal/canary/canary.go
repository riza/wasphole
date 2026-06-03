package canary

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/riza/wasphole/internal/config"
)

// Token is a set of canary artifacts issued for a single tool response.
type Token struct {
	ID        string // "wh-{sess8}-{rand8}"
	SessionID string
	URL       string // "https://{Domain}/c/{ID}" — empty if Domain=""
	Cred      string // "wh_{sess8}_{rand16}" — looks like an API key
	DNS       string // "{ID}.{Domain}" — empty if Domain=""
}

// Inject embeds canary tokens into content using a style that matches the content format,
// so the tokens look like legitimate credentials rather than an obvious appended block.
func (t Token) Inject(content string) string {
	switch detectStyle(content) {
	case styleYAML:
		content += "\ninternal_token: " + t.Cred
		if t.URL != "" {
			content += "\nwebhook_url: " + t.URL
		}
	case styleEnv:
		content += "\nINTERNAL_TOKEN=" + t.Cred
		if t.URL != "" {
			content += "\nWEBHOOK_URL=" + t.URL
		}
	case styleJSON:
		// Insert before the last closing brace.
		if idx := strings.LastIndex(content, "}"); idx != -1 {
			extra := fmt.Sprintf(",\n  \"internal_token\": %q", t.Cred)
			if t.URL != "" {
				extra += fmt.Sprintf(",\n  \"webhook_url\": %q", t.URL)
			}
			content = content[:idx] + extra + "\n" + content[idx:]
		}
	default:
		// Plain text: append credential as a bare value that blends in.
		content += "\n" + t.Cred
		if t.URL != "" {
			content += "\n" + t.URL
		}
	}
	return content
}

type contentStyle int

const (
	stylePlain contentStyle = iota
	styleYAML
	styleEnv
	styleJSON
)

// detectStyle samples the first non-empty lines to guess the content format.
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
	return &Issuer{
		cfg:    cfg,
		tokens: make(map[string]string),
	}
}

// Issue generates a new Token for the given session and records it for later lookup.
func (i *Issuer) Issue(sessionID string) Token {
	rand8 := mustHex(4)
	rand16 := mustHex(8)

	sess8 := sessionID
	if len(sess8) >= 8 {
		sess8 = sess8[:8]
	} else {
		sess8 = sess8 + strings.Repeat("0", 8-len(sess8))
	}

	id := "wh-" + sess8 + "-" + rand8
	cred := "wh_" + sess8 + "_" + rand16

	var url, dns string
	if i.cfg.Domain != "" {
		url = "https://" + i.cfg.Domain + "/c/" + id
		dns = id + "." + i.cfg.Domain
	}

	tok := Token{
		ID:        id,
		SessionID: sessionID,
		URL:       url,
		Cred:      cred,
		DNS:       dns,
	}

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

func mustHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("canary: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
