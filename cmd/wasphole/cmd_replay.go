package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/riza/wasphole/internal/session"
)

func runReplay() {
	dir := flag.String("dir", "./sessions", "session log directory")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: wasphole replay <session-id> [-dir ./sessions]")
		os.Exit(1)
	}
	sessionID := args[0]

	rs, err := session.ExportReplay(*dir, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	started := time.UnixMilli(rs.StartMs).UTC()
	dur := time.Duration(rs.DurMs) * time.Millisecond

	fmt.Printf("\nSession  %s\n", rs.SessionID)
	fmt.Printf("Started  %s UTC\n", started.Format("2006-01-02 15:04:05"))
	fmt.Printf("Duration %s  ·  %d events\n\n", fmtDur(dur), len(rs.Entries))
	fmt.Printf("  %-10s  %-8s  %-22s  %s\n", "Δ", "TYPE", "METHOD / TOOL", "DETAIL")
	fmt.Println("  " + strings.Repeat("─", 70))

	for _, e := range rs.Entries {
		delta := fmtDelta(e.DeltaMs)
		typ := e.Type
		method := e.Method
		detail := ""

		switch e.Type {
		case "request":
			if e.Method == "tools/call" {
				name, args := extractToolCall(e.Params)
				method = "tools/call  " + name
				detail = args
			}
		case "response":
			detail = summariseResult(e.Result)
		case "error":
			detail = "ERR " + e.Error
		}

		fmt.Printf("  %-10s  %-8s  %-22s  %s\n", delta, typ, method, truncate(detail, 60))
	}
	fmt.Println()
}

// extractToolCall pulls the tool name and a short param summary out of a tools/call params blob.
func extractToolCall(params any) (name, args string) {
	b, err := json.Marshal(params)
	if err != nil {
		return "?", ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "?", ""
	}
	// mcp-go wraps in {"params": {"name": ..., "arguments": ...}}
	if inner, ok := m["params"].(map[string]any); ok {
		m = inner
	}
	if n, ok := m["name"].(string); ok {
		name = n
	}
	if argsMap, ok := m["arguments"].(map[string]any); ok {
		var parts []string
		for k, v := range argsMap {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		args = strings.Join(parts, " ")
	}
	return
}

// summariseResult returns a short human-readable summary of a tool response.
func summariseResult(result any) string {
	if result == nil {
		return ""
	}
	b, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	s := string(b)
	// Try to extract the text content if it's an MCP result envelope.
	var m map[string]any
	if err := json.Unmarshal(b, &m); err == nil {
		if content, ok := m["content"].([]any); ok && len(content) > 0 {
			if item, ok := content[0].(map[string]any); ok {
				if text, ok := item["text"].(string); ok {
					lines := strings.Split(strings.TrimSpace(text), "\n")
					return fmt.Sprintf("%d lines", len(lines))
				}
			}
		}
	}
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

func fmtDelta(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("+%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("+%.1fs", d.Seconds())
	}
	return fmt.Sprintf("+%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
