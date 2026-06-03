package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runSessions() {
	dir := flag.String("dir", "./sessions", "session log directory")
	flag.Parse()

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read sessions dir %q: %v\n", *dir, err)
		os.Exit(1)
	}

	type row struct {
		id       string
		started  time.Time
		duration time.Duration
		events   int
	}

	var rows []row
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		first, last, count := readSessionBounds(filepath.Join(*dir, e.Name()))
		if count == 0 {
			continue
		}
		started := time.UnixMilli(first)
		dur := time.Duration(last-first) * time.Millisecond
		rows = append(rows, row{id, started, dur, count})
	}

	if len(rows) == 0 {
		fmt.Println("no sessions found in", *dir)
		return
	}

	fmt.Printf("%-32s  %-19s  %8s  %6s\n", "SESSION", "STARTED", "DURATION", "EVENTS")
	fmt.Println(strings.Repeat("─", 72))
	for _, r := range rows {
		fmt.Printf("%-32s  %-19s  %8s  %6d\n",
			r.id,
			r.started.UTC().Format("2006-01-02 15:04:05"),
			fmtDur(r.duration),
			r.events,
		)
	}
}

// readSessionBounds returns the first timestamp, last timestamp, and line count.
func readSessionBounds(path string) (first, last int64, count int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	type minEntry struct {
		TsMs int64 `json:"ts_ms"`
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e minEntry
		if err := json.Unmarshal(line, &e); err != nil || e.TsMs == 0 {
			continue
		}
		if count == 0 {
			first = e.TsMs
		}
		last = e.TsMs
		count++
	}
	return
}

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}
