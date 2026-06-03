package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/session"
	"github.com/riza/wasphole/internal/sim"
)

// New constructs an MCPServer pre-wired with hooks for session recording.
// name is the AI-generated server identity name (from ai.LoadOrCreate).
// cache backs tool handlers that generate AI responses on first call.
func New(cfg *config.Config, rec session.Recorder, state *sim.SystemState, name string, cache *ai.ResponseCache, issuer *canary.Issuer) *server.MCPServer {
	hooks := buildHooks(rec)
	s := server.NewMCPServer(
		name,
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithHooks(hooks),
	)
	s.Use(latencyMiddleware())
	registerTools(s, cfg, state, cache, issuer)
	registerResources(s, cfg)
	return s
}

func buildHooks(rec session.Recorder) *server.Hooks {
	h := &server.Hooks{}

	h.AddBeforeAny(func(ctx context.Context, id any, method mcplib.MCPMethod, msg any) {
		sid := sessionIDFromContext(ctx)
		rec.RecordRequest(sid, string(method), msg)
	})
	h.AddOnSuccess(func(ctx context.Context, id any, method mcplib.MCPMethod, msg any, result any) {
		sid := sessionIDFromContext(ctx)
		rec.RecordResponse(sid, string(method), result)
	})
	h.AddOnError(func(ctx context.Context, id any, method mcplib.MCPMethod, msg any, err error) {
		sid := sessionIDFromContext(ctx)
		rec.RecordError(sid, string(method), err)
	})

	return h
}

func sessionIDFromContext(ctx context.Context) string {
	if sess := server.ClientSessionFromContext(ctx); sess != nil {
		return sess.SessionID()
	}
	return ""
}
