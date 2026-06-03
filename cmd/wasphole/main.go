package main

import (
	"fmt"
	"os"
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

func main() {
	fmt.Fprint(os.Stderr, banner)

	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).With().Timestamp().Logger()

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:\n  wasphole server    [-config file]         run the honeypot server\n  wasphole sessions  [-dir ./sessions]       list recorded sessions\n  wasphole replay    <id> [-dir ./sessions]  show session timeline")
		os.Exit(1)
	}

	sub := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch sub {
	case "server":
		runServer()
	case "sessions":
		runSessions()
	case "replay":
		runReplay()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\nUsage:\n  wasphole server    [-config file]         run the honeypot server\n  wasphole sessions  [-dir ./sessions]       list recorded sessions\n  wasphole replay    <id> [-dir ./sessions]  show session timeline\n", sub)
		os.Exit(1)
	}
}
