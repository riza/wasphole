package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/alert"
	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/session"
	"github.com/riza/wasphole/internal/sim"
)

// New constructs an MCPServer pre-wired with hooks for session recording and alerting.
// name is the AI-generated server identity name (from ai.LoadOrCreate).
// cache backs tool handlers that generate AI responses on first call.
func New(cfg *config.Config, rec session.Recorder, state *sim.SystemState, name string, cache *ai.ResponseCache, issuer *canary.Issuer, engine *alert.Engine) *server.MCPServer {
	hooks := buildHooks(rec, engine)
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

func buildHooks(rec session.Recorder, engine *alert.Engine) *server.Hooks {
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

	// Session lifecycle → INFO alert + session state init
	h.AddOnRegisterSession(func(ctx context.Context, sess server.ClientSession) {
		sid := sess.SessionID()
		engine.InitSession(sid)
		engine.EmitConnection(sid)
	})
	h.AddOnUnregisterSession(func(ctx context.Context, sess server.ClientSession) {
		engine.FinalizeSession(sess.SessionID())
	})

	// Tool call success → behavioral observation
	h.AddAfterCallTool(func(ctx context.Context, id any, msg *mcplib.CallToolRequest, result any) {
		sid := sessionIDFromContext(ctx)
		if msg != nil {
			engine.ObserveSuccess(sid, msg.Params.Name, msg)
		}
	})

	// Tool call error → retry detection
	h.AddOnError(func(ctx context.Context, id any, method mcplib.MCPMethod, msg any, err error) {
		if method != "tools/call" {
			return
		}
		sid := sessionIDFromContext(ctx)
		engine.ObserveError(sid, toolNameFromMsg(msg))
	})

	return h
}

func toolNameFromMsg(msg any) string {
	switch v := msg.(type) {
	case *mcplib.CallToolRequest:
		if v != nil {
			return v.Params.Name
		}
	case mcplib.CallToolRequest:
		return v.Params.Name
	}
	return ""
}

func sessionIDFromContext(ctx context.Context) string {
	if sess := server.ClientSessionFromContext(ctx); sess != nil {
		return sess.SessionID()
	}
	return ""
}
