package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/riza/wasphole/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	log.Printf("wasphole starting: mode=%s transport=%s", cfg.Instance.Mode, cfg.Transport.Type)
}
