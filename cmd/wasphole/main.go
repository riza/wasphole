package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/alert"
	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/mcp"
	"github.com/riza/wasphole/internal/session"
	"github.com/riza/wasphole/internal/sim"
)

const banner = `
                .' '.            __
       .        .   .           (__\_
        .         .         . -{{_(|8)
          ' .  . ' ' .  . '     (__/

`

func main() {
	fmt.Fprint(os.Stderr, banner)

	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).With().Timestamp().Logger()

	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
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
		log.Info().Str("transport", "http").Str("addr", addr).Msg("wasphole ready")
		httpSrv := mcpserver.NewStreamableHTTPServer(srv)

		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()

		go func() {
			if err := httpSrv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("http server error")
			}
		}()

		<-sigCtx.Done()
		log.Info().Msg("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		rec.Close()
	}
}
