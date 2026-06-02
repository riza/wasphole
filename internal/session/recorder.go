package session

// Recorder captures MCP request/response lifecycle events for session logging.
type Recorder interface {
	RecordRequest(sessionID, method string, params any)
	RecordResponse(sessionID, method string, result any)
	RecordError(sessionID, method string, err error)
}

// NullRecorder discards all events. Used until the FileRecorder is wired (plan 02-03).
type NullRecorder struct{}

func (NullRecorder) RecordRequest(sessionID, method string, params any)  {}
func (NullRecorder) RecordResponse(sessionID, method string, result any) {}
func (NullRecorder) RecordError(sessionID, method string, err error)     {}
