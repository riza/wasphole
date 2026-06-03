package canary

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// alertEngine is a minimal interface to avoid import cycles.
type alertEngine interface {
	ObserveCanaryFire(sessionID, tokenID string)
}

// Listener serves /c/{tokenID} and fires attribution callbacks.
type Listener struct {
	issuer   *Issuer
	callback string // HTTPCallback URL — may be empty
	engine   alertEngine
}

// NewListener constructs a Listener backed by the given Issuer.
// engine may be nil if alert integration is not configured.
func NewListener(issuer *Issuer, httpCallback string, engine alertEngine) *Listener {
	return &Listener{
		issuer:   issuer,
		callback: httpCallback,
		engine:   engine,
	}
}

// Start begins listening on addr. Blocks until the server exits.
func (l *Listener) Start(addr string) error {
	return http.ListenAndServe(addr, l)
}

// ServeHTTP handles GET /c/{tokenID}.
func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/c/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	sessionID, _ := l.issuer.Lookup(id)

	if l.engine != nil {
		l.engine.ObserveCanaryFire(sessionID, id)
	}

	if l.callback != "" {
		go l.fireCallback(id, sessionID, r.RemoteAddr)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

type callbackPayload struct {
	TokenID    string `json:"token_id"`
	SessionID  string `json:"session_id"`
	RemoteAddr string `json:"remote_addr"`
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
