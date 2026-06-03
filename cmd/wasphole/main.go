package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/mcp"
	"github.com/riza/wasphole/internal/session"
	"github.com/riza/wasphole/internal/sim"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	rec, err := session.NewFileRecorder(cfg.Session.LogDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	state, err := sim.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: sim init failed: %v\n", err)
		os.Exit(1)
	}

	cacheDir := cfg.AI.CacheDir
	if cacheDir == "" {
		cacheDir = "./cache"
	}

	aiClient, err := ai.NewClient(cfg.AI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: ai init: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	identity, err := ai.LoadOrCreate(ctx, aiClient, cacheDir, cfg.Instance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: identity: %v\n", err)
		os.Exit(1)
	}

	srv := mcp.New(cfg, rec, state, identity.ServerName)

	switch cfg.Transport.Type {
	case "stdio":
		log.Printf("wasphole starting: mode=%s transport=stdio", cfg.Instance.Mode)
		if err := mcpserver.ServeStdio(srv); err != nil {
			log.Printf("stdio: %v", err)
		}
	case "http":
		addr := fmt.Sprintf(":%d", cfg.Transport.Port)
		log.Printf("wasphole starting: mode=%s transport=http addr=%s", cfg.Instance.Mode, addr)
		httpSrv := mcpserver.NewStreamableHTTPServer(srv)
		if err := httpSrv.Start(addr); err != nil {
			log.Printf("http: %v", err)
		}
	}
}
