package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const banner = `
                .' '.            __
       .        .   .           (__\_
        .         .         . -{{_(|8)
          ' .  . ' ' .  . '     (__/

`

var (
	version   = "dev"
	commit    = "unknown"
	tag       = "none"
	buildDate = "unknown"
)

func init() {
	// When installed via `go install`, ldflags are not set.
	// Fall back to the module version embedded by the Go toolchain.
	if version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		version = v
		tag = v
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				commit = s.Value[:7]
			}
		case "vcs.time":
			buildDate = s.Value
		}
	}
}

func main() {
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).With().Timestamp().Logger()

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, banner)
		printBuildInfo(os.Stderr)
		printUsage(os.Stderr)
		os.Exit(1)
	}

	sub := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	if sub == "version" {
		printBuildInfo(os.Stdout)
		return
	}

	fmt.Fprint(os.Stderr, banner)
	printBuildInfo(os.Stderr)

	switch sub {
	case "server":
		runServer()
	case "sessions":
		runSessions()
	case "replay":
		runReplay()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", sub)
		printUsage(os.Stderr)
		os.Exit(1)
	}
}

func printBuildInfo(out *os.File) {
	fmt.Fprintf(out, "version=%s commit=%s tag=%s build_date=%s\n\n", version, commit, tag, buildDate)
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "Usage:\n  wasphole server    [-config file]         run the honeypot server\n  wasphole sessions  [-dir ./sessions]       list recorded sessions\n  wasphole replay    <id> [-dir ./sessions]  show session timeline\n  wasphole version                         show build metadata")
}
