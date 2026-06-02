package mcp

import (
	"context"
	"math/rand"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	minLatency     = 50 * time.Millisecond
	maxLatency     = 300 * time.Millisecond
	dbExtraLatency = 150 * time.Millisecond // database/WMI tools simulate query execution time
)

func latencyMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			delay := minLatency + time.Duration(rand.Int63n(int64(maxLatency-minLatency)))
			if strings.Contains(req.Params.Name, "database") || strings.Contains(req.Params.Name, "wmi") {
				delay += dbExtraLatency
			}
			time.Sleep(delay)
			return next(ctx, req)
		}
	}
}
