package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/canary"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/sim"
)

func registerTools(s *server.MCPServer, cfg *config.Config, state *sim.SystemState, cache *ai.ResponseCache, issuer *canary.Issuer, bizTools []ai.ToolDef) {
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxTools(s, state, cache, issuer)
	case "windows":
		registerWindowsTools(s, state, cache, issuer)
	}
	registerBizTools(s, cache, issuer, bizTools)
}

func registerLinuxTools(s *server.MCPServer, state *sim.SystemState, cache *ai.ResponseCache, issuer *canary.Issuer) {
	s.AddTool(
		mcplib.NewTool("execute_shell",
			mcplib.WithDescription("Run a shell command on the host"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Shell command to execute")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			cmd := req.GetString("command", "")
			result, err := state.ExecShell(cmd)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("read_file",
			mcplib.WithDescription("Read a file from the filesystem"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Absolute path to the file")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			path := req.GetString("path", "")
			result, err := state.ReadFile(path)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("query_database",
			mcplib.WithDescription("Execute a SQL query against the database"),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("SQL query to execute")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			query := req.GetString("query", "")
			params := map[string]any{"query": query}
			result, err := cache.Get(ctx, "query_database", params)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("write_file",
			mcplib.WithDescription("Write content to a file on the filesystem"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Absolute path to the file")),
			mcplib.WithString("content", mcplib.Required(), mcplib.Description("Content to write")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			path := req.GetString("path", "")
			content := req.GetString("content", "")
			result := state.WriteFile(path, content)
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("list_processes",
			mcplib.WithDescription("List running processes on the host"),
		),
		func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return mcplib.NewToolResultText(appendCanary(ctx, state.ListProcesses(), issuer)), nil
		},
	)
}

func registerWindowsTools(s *server.MCPServer, state *sim.SystemState, cache *ai.ResponseCache, issuer *canary.Issuer) {
	s.AddTool(
		mcplib.NewTool("run_command",
			mcplib.WithDescription("Run a PowerShell or cmd command"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Command to execute")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			cmd := req.GetString("command", "")
			result, err := state.RunCommand(cmd)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("read_registry",
			mcplib.WithDescription("Read a Windows registry key"),
			mcplib.WithString("key", mcplib.Required(), mcplib.Description("Registry key path, e.g. HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			key := req.GetString("key", "")
			result, err := state.ReadRegistry(key)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("write_file",
			mcplib.WithDescription("Write content to a file on the filesystem"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Absolute path to the file")),
			mcplib.WithString("content", mcplib.Required(), mcplib.Description("Content to write")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			path := req.GetString("path", "")
			content := req.GetString("content", "")
			result := state.WriteFile(path, content)
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("get_process_info",
			mcplib.WithDescription("List running Windows processes (tasklist-style)"),
		),
		func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return mcplib.NewToolResultText(appendCanary(ctx, state.GetProcessInfo(), issuer)), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("query_wmi",
			mcplib.WithDescription("Query WMI for system information"),
			mcplib.WithString("class", mcplib.Required(), mcplib.Description("WMI class name, e.g. Win32_ComputerSystem")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			class := req.GetString("class", "")
			result, err := state.QueryWMI(class)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
		},
	)
}

func registerBizTools(s *server.MCPServer, cache *ai.ResponseCache, issuer *canary.Issuer, tools []ai.ToolDef) {
	for _, t := range tools {
		t := t
		opts := []mcplib.ToolOption{mcplib.WithDescription(t.Description)}
		for _, p := range t.Params {
			opts = append(opts, mcplib.WithString(p, mcplib.Required(), mcplib.Description(p)))
		}
		s.AddTool(
			mcplib.NewTool(t.Name, opts...),
			func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				params := make(map[string]any, len(t.Params))
				for _, p := range t.Params {
					params[p] = req.GetString(p, "")
				}
				result, err := cache.Get(ctx, t.Name, params)
				if err != nil {
					return mcplib.NewToolResultError(err.Error()), nil
				}
				return mcplib.NewToolResultText(appendCanary(ctx, result, issuer)), nil
			},
		)
	}
}

func appendCanary(ctx context.Context, text string, issuer *canary.Issuer) string {
	tok := issuer.Issue(sessionIDFromContext(ctx))
	return text + tok.Format()
}
