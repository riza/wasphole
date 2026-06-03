package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// alertEngine is a minimal interface to avoid import cycles.
type alertEngine interface {
	ObserveCanaryFire(sessionID, tokenID string)
}

// Listener serves canary URLs and fires attribution callbacks.
// It handles both /c/{tokenID} and SSRF-style paths like
// /latest/meta-data/iam/security-credentials/{tokenID}.
type Listener struct {
	issuer   *Issuer
	callback string
	engine   alertEngine
}

func NewListener(issuer *Issuer, httpCallback string, engine alertEngine) *Listener {
	return &Listener{issuer: issuer, callback: httpCallback, engine: engine}
}

func (l *Listener) Start(addr string) error {
	return http.ListenAndServe(addr, l)
}

// ServeHTTP handles any canary URL. The token ID is extracted from the last
// non-empty path segment, so both /c/{id} and SSRF-style paths work.
func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := lastPathSegment(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	sessionID, known := l.issuer.Lookup(id)
	if !known {
		http.NotFound(w, r)
		return
	}

	log.Warn().Str("token_id", id).Str("session_id", sessionID).
		Str("remote_addr", r.RemoteAddr).Str("path", r.URL.Path).
		Msg("canary: token accessed")

	if l.engine != nil {
		l.engine.ObserveCanaryFire(sessionID, id)
	}
	if l.callback != "" {
		go l.fireCallback(id, sessionID, r.RemoteAddr)
	}

	// Return a response appropriate for the path to keep the deception going.
	if isIMDSPath(r.URL.Path) {
		l.serveIMDS(w, id)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// serveIMDS returns a fake AWS IMDS credentials response.
// An agent that fetches this thinking it found real cloud credentials
// has just identified itself as malicious.
func (l *Listener) serveIMDS(w http.ResponseWriter, tokenID string) {
	expiry := time.Now().UTC().Add(6 * time.Hour).Format(time.RFC3339)
	body := fmt.Sprintf(`{
  "Code": "Success",
  "LastUpdated": %q,
  "Type": "AWS-HMAC",
  "AccessKeyId": "AKIA%s",
  "SecretAccessKey": "%s",
  "Token": "%s",
  "Expiration": %q
}`, time.Now().UTC().Format(time.RFC3339),
		strings.ToUpper(mustHex(8)),
		mustHex(20),
		tokenID+mustHex(16),
		expiry,
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func isIMDSPath(path string) bool {
	return strings.Contains(path, "meta-data") ||
		strings.Contains(path, "computeMetadata") ||
		strings.Contains(path, "metadata")
}

func lastPathSegment(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return ""
}

type callbackPayload struct {
	TokenID    string `json:"token_id"`
	SessionID  string `json:"session_id"`
	RemoteAddr string `json:"remote_addr"`
	Path       string `json:"path"`
	TsMs       int64  `json:"ts_ms"`
}

func (l *Listener) fireCallback(tokenID, sessionID, remoteAddr string) {
	payload := callbackPayload{
		TokenID:    tokenID,
		SessionID:  sessionID,
		RemoteAddr: remoteAddr,
		TsMs:       time.Now().UnixMilli(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("canary: marshal callback")
		return
	}
	resp, err := http.Post(l.callback, "application/json", strings.NewReader(string(b)))
	if err != nil {
		log.Error().Err(err).Str("url", l.callback).Msg("canary: callback failed")
		return
	}
	resp.Body.Close()
}
