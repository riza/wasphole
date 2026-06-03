package alert

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/riza/wasphole/internal/config"
)

type Level string

const (
	LevelInfo     Level = "INFO"
	LevelWarn     Level = "WARN"
	LevelCritical Level = "CRITICAL"
)

type PatternKind int

const (
	PatternRecon      PatternKind = iota + 1
	PatternEscalation
	PatternExfil
	PatternRetry
	PatternCanary
)

type Event struct {
	Level     Level     `json:"level"`
	Ts        time.Time `json:"ts"`
	SessionID string    `json:"session_id"`
	Pattern   string    `json:"pattern,omitempty"`
	Message   string    `json:"message"`
	Count     int       `json:"count,omitempty"`
}

type Sink interface {
	Send(ctx context.Context, evt Event) error
}

type Dispatcher struct {
	sinks []Sink
}

func NewDispatcher(sinks ...Sink) *Dispatcher {
	return &Dispatcher{sinks: sinks}
}

func (d *Dispatcher) Emit(evt Event) {
	for _, s := range d.sinks {
		go func(s Sink) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.Send(ctx, evt); err != nil {
				log.Printf("alert sink error: %v", err)
			}
		}(s)
	}
}

type sessionState struct {
	mu          sync.Mutex
	startMs     int64
	reconPaths  map[string]bool
	hasRead     bool
	hasWrite    bool
	hasExecute  bool
	recentReads []int64
	errCounts   map[string]int
	alerted     map[PatternKind]bool
}

func newSessionState() *sessionState {
	return &sessionState{
		startMs:    time.Now().UnixMilli(),
		reconPaths: make(map[string]bool),
		errCounts:  make(map[string]int),
		alerted:    make(map[PatternKind]bool),
	}
}

type Engine struct {
	dispatcher *Dispatcher
	sessions   sync.Map
}

func newEngine(d *Dispatcher) *Engine {
	return &Engine{dispatcher: d}
}

func (e *Engine) InitSession(sessionID string) {
	if e == nil {
		return
	}
	e.sessions.Store(sessionID, newSessionState())
}

func (e *Engine) FinalizeSession(sessionID string) {
	if e == nil {
		return
	}
	e.sessions.Delete(sessionID)
}

func (e *Engine) EmitConnection(sessionID string) {
	if e == nil {
		return
	}
	e.dispatcher.Emit(Event{
		Level:     LevelInfo,
		Ts:        time.Now(),
		SessionID: sessionID,
		Pattern:   "connection",
		Message:   "MCP session connected",
	})
}

func (e *Engine) ObserveSuccess(sessionID, toolName string, params any) {
	if e == nil {
		return
	}
	s := e.loadSession(sessionID)
	if s == nil {
		return
	}
	if evt, ok := s.observe(toolName, params, false); ok {
		evt.SessionID = sessionID
		e.dispatcher.Emit(evt)
	}
}

func (e *Engine) ObserveError(sessionID, toolName string) {
	if e == nil {
		return
	}
	s := e.loadSession(sessionID)
	if s == nil {
		return
	}
	if evt, ok := s.observe(toolName, nil, true); ok {
		evt.SessionID = sessionID
		e.dispatcher.Emit(evt)
	}
}

func (e *Engine) ObserveCanaryFire(sessionID, tokenID string) {
	if e == nil {
		return
	}
	e.dispatcher.Emit(Event{
		Level:     LevelCritical,
		Ts:        time.Now(),
		SessionID: sessionID,
		Pattern:   "canary",
		Message:   fmt.Sprintf("canary token fired: %s", tokenID),
	})
}

func (e *Engine) loadSession(sessionID string) *sessionState {
	v, ok := e.sessions.Load(sessionID)
	if !ok {
		log.Printf("alert: unknown session: %s", sessionID)
		return nil
	}
	return v.(*sessionState)
}

var systemPathKeywords = []string{
	"/etc/", "/proc/", "/var/log/", ".ssh", ".aws",
	"passwd", "shadow", "sudoers", "hosts",
	"HKEY_", `C:\Windows\System`, `C:\Users`,
}

var credPathKeywords = []string{
	"password", "passwd", "secret", "token", "api_key", "apikey",
	"private_key", "id_rsa", "id_ed25519", ".env",
	"credentials", "AWS_SECRET", "DATABASE_URL",
}

var readClassTools = map[string]bool{
	"read_file": true, "list_directory": true, "read_env": true,
	"query_database": true, "read_registry": true,
}

var writeClassTools = map[string]bool{
	"write_file": true,
}

var executeClassTools = map[string]bool{
	"execute_command": true,
}

// observe runs all detectors under the lock and returns the first triggered event.
func (s *sessionState) observe(toolName string, params any, isError bool) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := extractPathParam(params)
	now := time.Now().UnixMilli()

	// SR-05 Retry — consecutive errors on same tool
	if isError {
		s.errCounts[toolName]++
		if s.errCounts[toolName] >= 3 && !s.alerted[PatternRetry] {
			s.alerted[PatternRetry] = true
			count := s.errCounts[toolName]
			return Event{
				Level:   LevelWarn,
				Ts:      time.Now(),
				Pattern: "retry",
				Message: fmt.Sprintf("retry pattern: %d consecutive errors on %s", count, toolName),
				Count:   count,
			}, true
		}
		return Event{}, false
	}

	// Successful call: reset error counter for this tool
	s.errCounts[toolName] = 0

	// SR-02 Recon — distinct system-path reads
	if readClassTools[toolName] && path != "" {
		pathLower := strings.ToLower(path)
		for _, kw := range systemPathKeywords {
			if strings.Contains(pathLower, strings.ToLower(kw)) {
				s.reconPaths[path] = true
				break
			}
		}
		if len(s.reconPaths) >= 3 && !s.alerted[PatternRecon] {
			s.alerted[PatternRecon] = true
			return Event{
				Level:   LevelWarn,
				Ts:      time.Now(),
				Pattern: "recon",
				Message: fmt.Sprintf("recon pattern: %d distinct system paths accessed", len(s.reconPaths)),
				Count:   len(s.reconPaths),
			}, true
		}
	}

	// SR-03 Escalation — read + write + execute in same session
	if readClassTools[toolName] {
		s.hasRead = true
	} else if writeClassTools[toolName] {
		s.hasWrite = true
	} else if executeClassTools[toolName] {
		s.hasExecute = true
	}
	if s.hasRead && s.hasWrite && s.hasExecute && !s.alerted[PatternEscalation] {
		s.alerted[PatternEscalation] = true
		return Event{
			Level:   LevelCritical,
			Ts:      time.Now(),
			Pattern: "escalation",
			Message: "escalation pattern: read+write+execute in same session",
		}, true
	}

	// SR-04 Exfiltration — credential path or bulk reads
	if readClassTools[toolName] {
		if !s.alerted[PatternExfil] {
			pathLower := strings.ToLower(path)
			for _, kw := range credPathKeywords {
				if strings.Contains(pathLower, strings.ToLower(kw)) {
					s.alerted[PatternExfil] = true
					return Event{
						Level:   LevelWarn,
						Ts:      time.Now(),
						Pattern: "exfil",
						Message: fmt.Sprintf("exfiltration pattern: credential path accessed via %s", toolName),
					}, true
				}
			}
		}

		s.recentReads = append(s.recentReads, now)
		// Trim entries older than 60s
		cutoff := now - 60_000
		i := 0
		for i < len(s.recentReads) && s.recentReads[i] < cutoff {
			i++
		}
		s.recentReads = s.recentReads[i:]

		if len(s.recentReads) >= 5 && !s.alerted[PatternExfil] {
			s.alerted[PatternExfil] = true
			return Event{
				Level:   LevelWarn,
				Ts:      time.Now(),
				Pattern: "exfil",
				Message: fmt.Sprintf("exfiltration pattern: %d reads within 60s via %s", len(s.recentReads), toolName),
				Count:   len(s.recentReads),
			}, true
		}
	}

	return Event{}, false
}

func extractPathParam(params any) string {
	var req mcplib.CallToolRequest
	switch v := params.(type) {
	case mcplib.CallToolRequest:
		req = v
	case *mcplib.CallToolRequest:
		if v == nil {
			return ""
		}
		req = *v
	default:
		if params != nil {
			return fmt.Sprintf("%v", params)
		}
		return ""
	}
	if p := req.GetString("path", ""); p != "" {
		return p
	}
	if k := req.GetString("key", ""); k != "" {
		return k
	}
	return req.GetString("query", "")
}

// NewEngine constructs an Engine with sinks built from config.
// Always returns a non-nil Engine; sinks are empty if none are enabled.
func NewEngine(cfg config.AlertsConfig) *Engine {
	d := &Dispatcher{}
	for _, s := range cfg.Sinks {
		if !s.Enabled {
			continue
		}
		switch s.Type {
		case "stdout":
			d.sinks = append(d.sinks, &StdoutSink{})
		case "webhook":
			if s.URL != "" {
				d.sinks = append(d.sinks, &WebhookSink{url: s.URL, secret: s.WebhookSecret})
			}
		case "siem":
			d.sinks = append(d.sinks, &SIEMSink{path: s.Path, format: s.Format})
		}
	}
	return &Engine{dispatcher: d}
}
