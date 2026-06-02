package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Recorder captures MCP request/response lifecycle events for session logging.
type Recorder interface {
	RecordRequest(sessionID, method string, params any)
	RecordResponse(sessionID, method string, result any)
	RecordError(sessionID, method string, err error)
}

// NullRecorder discards all events.
type NullRecorder struct{}

func (NullRecorder) RecordRequest(sessionID, method string, params any)  {}
func (NullRecorder) RecordResponse(sessionID, method string, result any) {}
func (NullRecorder) RecordError(sessionID, method string, err error)     {}

// LogEntry is one line in the JSONL session log.
type LogEntry struct {
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Method    string `json:"method"`
	TsMs      int64  `json:"ts_ms"`
	Params    any    `json:"params,omitempty"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FileRecorder writes JSONL session logs to a directory, one file per session.
// Safe for concurrent use from multiple hooks.
type FileRecorder struct {
	logDir string
	mu     sync.Mutex
	files  map[string]*os.File
}

// NewFileRecorder creates a FileRecorder that writes to logDir.
// The directory is created if it does not exist.
func NewFileRecorder(logDir string) (*FileRecorder, error) {
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, fmt.Errorf("session: create log dir: %w", err)
	}
	return &FileRecorder{logDir: logDir, files: make(map[string]*os.File)}, nil
}

func (r *FileRecorder) RecordRequest(sessionID, method string, params any) {
	r.write(LogEntry{SessionID: sessionID, Type: "request", Method: method, TsMs: time.Now().UnixMilli(), Params: params})
}

func (r *FileRecorder) RecordResponse(sessionID, method string, result any) {
	r.write(LogEntry{SessionID: sessionID, Type: "response", Method: method, TsMs: time.Now().UnixMilli(), Result: result})
}

func (r *FileRecorder) RecordError(sessionID, method string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	r.write(LogEntry{SessionID: sessionID, Type: "error", Method: method, TsMs: time.Now().UnixMilli(), Error: msg})
}

func (r *FileRecorder) write(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return // don't block the MCP response path on marshal failure
	}

	sid := entry.SessionID
	if sid == "" {
		sid = "unknown"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	f, ok := r.files[sid]
	if !ok {
		path := filepath.Join(r.logDir, sid+".jsonl")
		f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return
		}
		r.files[sid] = f
	}

	_, _ = f.Write(append(data, '\n'))
}

// Close flushes and closes all open session log files.
func (r *FileRecorder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.files {
		f.Close()
	}
	r.files = make(map[string]*os.File)
}
