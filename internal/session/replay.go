package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReplayEntry wraps LogEntry with delta timing from session start.
type ReplayEntry struct {
	LogEntry
	DeltaMs int64 `json:"delta_ms"`
}

// ReplaySession is a complete session with per-entry timing data.
type ReplaySession struct {
	SessionID string        `json:"session_id"`
	StartMs   int64         `json:"start_ms"`
	DurMs     int64         `json:"dur_ms"`
	Entries   []ReplayEntry `json:"entries"`
}

// ExportReplay reads the JSONL log and returns a ReplaySession with delta timing.
func ExportReplay(logDir, sessionID string) (*ReplaySession, error) {
	sess, err := Export(logDir, sessionID)
	if err != nil {
		return nil, err
	}
	rs := &ReplaySession{SessionID: sess.SessionID}
	if len(sess.Entries) > 0 {
		rs.StartMs = sess.Entries[0].TsMs
		rs.DurMs = sess.Entries[len(sess.Entries)-1].TsMs - rs.StartMs
	}
	for _, e := range sess.Entries {
		rs.Entries = append(rs.Entries, ReplayEntry{
			LogEntry: e,
			DeltaMs:  e.TsMs - rs.StartMs,
		})
	}
	return rs, nil
}

// Session is the structured representation of a complete MCP session.
type Session struct {
	SessionID string     `json:"session_id"`
	Entries   []LogEntry `json:"entries"`
}

// Export reads the JSONL log for sessionID and returns a Session struct.
// Returns an error if the log file does not exist.
func Export(logDir, sessionID string) (*Session, error) {
	if sessionID == "" {
		sessionID = "unknown"
	}
	path := filepath.Join(logDir, sessionID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("session: open log %s: %w", path, err)
	}
	defer f.Close()

	sess := &Session{SessionID: sessionID}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}
		sess.Entries = append(sess.Entries, entry)
	}
	return sess, scanner.Err()
}
