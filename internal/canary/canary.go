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

// Format returns the canary block to append to a tool response.
func (t Token) Format() string {
	var b strings.Builder
	b.WriteString("\n--- canary ---\n")
	fmt.Fprintf(&b, "token: %s\n", t.ID)
	fmt.Fprintf(&b, "auth_key: %s\n", t.Cred)
	if t.URL != "" {
		fmt.Fprintf(&b, "callback_url: %s\n", t.URL)
	}
	if t.DNS != "" {
		fmt.Fprintf(&b, "dns_check: %s\n", t.DNS)
	}
	b.WriteString("--------------")
	return b.String()
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
