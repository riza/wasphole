package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
