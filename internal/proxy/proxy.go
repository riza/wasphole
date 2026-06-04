package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

type scanner interface {
	Scan(text string) (sessionID, tokenID string, ok bool)
}

type alertEngine interface {
	ObserveCanaryFire(sessionID, tokenID string)
}

// Proxy is an OpenAI-compatible HTTP reverse proxy that scans incoming
// chat completion requests for canary credentials before forwarding.
type Proxy struct {
	issuer   scanner
	engine   alertEngine
	upstream *httputil.ReverseProxy
}

// New constructs a Proxy. upstreamURL is the LLM API to forward to
// (e.g. "https://api.openai.com"). If empty, requests are not forwarded
// and a minimal stub response is returned instead.
func New(issuer scanner, engine alertEngine, upstreamURL string) (*Proxy, error) {
	p := &Proxy{issuer: issuer, engine: engine}
	if upstreamURL != "" {
		u, err := url.Parse(upstreamURL)
		if err != nil {
			return nil, err
		}
		rp := httputil.NewSingleHostReverseProxy(u)
		orig := rp.Director
		rp.Director = func(r *http.Request) {
			orig(r)
			r.Host = u.Host
		}
		p.upstream = rp
	}
	return p, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	if sessID, tokID, ok := p.issuer.Scan(string(body)); ok {
		log.Warn().Str("session_id", sessID).Str("token_id", tokID).
			Str("remote_addr", r.RemoteAddr).
			Msg("proxy: canary credential observed in outbound model prompt")
		if p.engine != nil {
			p.engine.ObserveCanaryFire(sessID, tokID)
		}
	}

	if p.upstream != nil {
		p.upstream.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"id":"stub","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)
}

func (p *Proxy) Start(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
	}
	return srv.ListenAndServe()
}
