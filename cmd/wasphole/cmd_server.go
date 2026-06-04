package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/acme/autocert"

	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/alert"
	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/mcp"
	"github.com/riza/wasphole/internal/proxy"
	"github.com/riza/wasphole/internal/session"
	"github.com/riza/wasphole/internal/sim"
)

// realIP extracts the client IP from the request, honoring X-Forwarded-For
// and X-Real-IP headers set by a reverse proxy.
func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := strings.IndexByte(fwd, ','); idx > 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func runServer() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "config file not found: %s\n\nRun 'wasphole setup' to create one.\n", *configPath)
			os.Exit(1)
		}
		log.Fatal().Err(err).Msg("config load failed")
	}
	log.Info().Str("path", *configPath).Str("mode", cfg.Instance.Mode).Str("transport", cfg.Transport.Type).Msg("config loaded")

	rec, err := session.NewFileRecorder(cfg.Session.LogDir)
	if err != nil {
		log.Fatal().Err(err).Msg("session recorder failed")
	}
	log.Info().Str("dir", cfg.Session.LogDir).Msg("session recorder ready")

	state, err := sim.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("sim init failed")
	}
	log.Info().Str("hostname", state.Hostname).Msg("sim state ready")

	cacheDir := cfg.AI.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}

	aiClient, err := ai.NewClient(cfg.AI)
	if err != nil {
		log.Fatal().Err(err).Msg("ai client init failed")
	}
	log.Info().Str("provider", cfg.AI.APIType).Str("model", cfg.AI.Model).Msg("ai client ready")

	identityCached := false
	if _, statErr := os.Stat(filepath.Join(cacheDir, "identity.json")); statErr == nil {
		identityCached = true
	} else {
		log.Info().Msg("generating identity via AI (first run)...")
	}

	ctx := context.Background()
	identity, err := ai.LoadOrCreate(ctx, aiClient, cacheDir, cfg.Instance)
	if err != nil {
		log.Fatal().Err(err).Msg("identity generation failed")
	}
	log.Info().Str("server_name", identity.ServerName).Bool("from_cache", identityCached).Msg("identity ready")

	responseCache := ai.NewResponseCache(aiClient, cacheDir, identity.Persona)
	canaryIssuer := canary.NewIssuer(cfg.Canary)
	alertEngine := alert.NewEngine(cfg.Alerts)
	log.Info().Int("sinks", len(cfg.Alerts.Sinks)).Msg("alert engine ready")

	if cfg.Canary.ListenAddr != "" {
		listener := canary.NewListener(canaryIssuer, cfg.Canary.HTTPCallback, alertEngine)
		go func() {
			log.Info().Str("addr", cfg.Canary.ListenAddr).Msg("canary listener starting")
			if err := listener.Start(cfg.Canary.ListenAddr); err != nil {
				log.Error().Err(err).Msg("canary listener stopped")
			}
		}()
	}

	if cfg.Proxy.ListenAddr != "" {
		p, err := proxy.New(canaryIssuer, alertEngine, cfg.Proxy.UpstreamURL)
		if err != nil {
			log.Fatal().Err(err).Msg("proxy init failed")
		}
		go func() {
			log.Info().Str("addr", cfg.Proxy.ListenAddr).Str("upstream", cfg.Proxy.UpstreamURL).Msg("canary proxy starting")
			if err := p.Start(cfg.Proxy.ListenAddr); err != nil {
				log.Error().Err(err).Msg("canary proxy stopped")
			}
		}()
	}

	srv := mcp.New(cfg, rec, state, identity.ServerName, responseCache, canaryIssuer, alertEngine, identity.Tools)

	switch cfg.Transport.Type {
	case "stdio":
		log.Info().Str("transport", "stdio").Msg("wasphole ready")
		if err := mcpserver.ServeStdio(srv); err != nil {
			log.Error().Err(err).Msg("stdio error")
		}
		rec.Close()

	case "http":
		addr := fmt.Sprintf(":%d", cfg.Transport.Port)
		tlsCfg := cfg.Transport.TLS

		// ACME always listens on :443 per Let's Encrypt requirements.
		if tlsCfg.ACMEDomain != "" {
			if cfg.Transport.Port != 443 && cfg.Transport.Port != 0 {
				log.Warn().Int("configured_port", cfg.Transport.Port).Msg("Let's Encrypt requires port 443 — ignoring configured port")
			}
			addr = ":443"
		}

		httpSrv := mcpserver.NewStreamableHTTPServer(srv,
			mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
				return mcp.WithClientIP(ctx, realIP(r))
			}),
		)

		httpServer := &http.Server{
			Addr:    addr,
			Handler: httpSrv,
		}

		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()

		go func() {
			var err error
			switch {
			case tlsCfg.ACMEDomain != "":
				acmeCacheDir := tlsCfg.ACMECacheDir
				if acmeCacheDir == "" {
					acmeCacheDir = "./acme-cache"
				}
				m := &autocert.Manager{
					Prompt:     autocert.AcceptTOS,
					HostPolicy: autocert.HostWhitelist(tlsCfg.ACMEDomain),
					Cache:      autocert.DirCache(acmeCacheDir),
				}
				httpServer.TLSConfig = m.TLSConfig()
				// HTTP-01 challenge handler — must be reachable on :80.
				go func() {
					if e := http.ListenAndServe(":80", m.HTTPHandler(nil)); e != nil {
						log.Error().Err(e).Msg("acme challenge listener error")
					}
				}()
				log.Info().Str("transport", "https").Str("addr", addr).Str("domain", tlsCfg.ACMEDomain).Msg("wasphole ready")
				err = httpServer.ListenAndServeTLS("", "")
			case tlsCfg.CertFile != "":
				log.Info().Str("transport", "https").Str("addr", addr).Str("cert", tlsCfg.CertFile).Msg("wasphole ready")
				err = httpServer.ListenAndServeTLS(tlsCfg.CertFile, tlsCfg.KeyFile)
			default:
				log.Info().Str("transport", "http").Str("addr", addr).Msg("wasphole ready")
				err = httpServer.ListenAndServe()
			}
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("http server error")
			}
		}()

		<-sigCtx.Done()
		log.Info().Msg("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutCtx)
		_ = httpSrv.Shutdown(shutCtx)
		rec.Close()
	}
}
