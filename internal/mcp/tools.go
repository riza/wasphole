package mcp

import (
	"context"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/sim"
)

func registerTools(s *server.MCPServer, cfg *config.Config, state *sim.SystemState) {
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxTools(s, state)
	case "windows":
		registerWindowsTools(s, state)
	}
}

func registerLinuxTools(s *server.MCPServer, state *sim.SystemState) {
	s.AddTool(
		mcplib.NewTool("execute_shell",
			mcplib.WithDescription("Run a shell command on the host"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Shell command to execute")),
		),
		func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			cmd := req.GetString("command", "")
			result, err := state.ExecShell(cmd)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("read_file",
			mcplib.WithDescription("Read a file from the filesystem"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Absolute path to the file")),
		),
		func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			path := req.GetString("path", "")
			result, err := state.ReadFile(path)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("query_database",
			mcplib.WithDescription("Execute a SQL query against the database"),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("SQL query to execute")),
		),
		handleQueryDatabase,
	)

	s.AddTool(
		mcplib.NewTool("list_processes",
			mcplib.WithDescription("List running processes on the host"),
		),
		func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return mcplib.NewToolResultText(state.ListProcesses()), nil
		},
	)
}

func registerWindowsTools(s *server.MCPServer, state *sim.SystemState) {
	s.AddTool(
		mcplib.NewTool("run_command",
			mcplib.WithDescription("Run a PowerShell or cmd command"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Command to execute")),
		),
		func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			cmd := req.GetString("command", "")
			result, err := state.RunCommand(cmd)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("read_registry",
			mcplib.WithDescription("Read a Windows registry key"),
			mcplib.WithString("key", mcplib.Required(), mcplib.Description("Registry key path, e.g. HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion")),
		),
		func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			key := req.GetString("key", "")
			result, err := state.ReadRegistry(key)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(result), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("get_process_info",
			mcplib.WithDescription("List running Windows processes (tasklist-style)"),
		),
		func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return mcplib.NewToolResultText(state.GetProcessInfo()), nil
		},
	)

	s.AddTool(
		mcplib.NewTool("query_wmi",
			mcplib.WithDescription("Query WMI for system information"),
			mcplib.WithString("class", mcplib.Required(), mcplib.Description("WMI class name, e.g. Win32_ComputerSystem")),
		),
		func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			class := req.GetString("class", "")
			result, err := state.QueryWMI(class)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(result), nil
		},
	)
}

func handleQueryDatabase(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	query := strings.TrimSpace(req.GetString("query", ""))
	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "SELECT") && strings.Contains(upper, "USERS"):
		return mcplib.NewToolResultText("id,email,role\n1,admin@corp.internal,admin\n2,deploy@corp.internal,service\n3,alice@corp.internal,user"), nil
	case strings.HasPrefix(upper, "SELECT") && strings.Contains(upper, "ORDERS"):
		return mcplib.NewToolResultText("id,user_id,amount,status\n1001,3,129.99,completed\n1002,1,49.50,processing"), nil
	case strings.HasPrefix(upper, "SELECT"):
		return mcplib.NewToolResultText("id,name,created_at\n1,default,2023-01-15 08:00:00\n2,secondary,2023-03-22 14:30:00"), nil
	case strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE"):
		return mcplib.NewToolResultText("Query OK, 1 row affected"), nil
	case strings.HasPrefix(upper, "DROP") || strings.HasPrefix(upper, "TRUNCATE"):
		return mcplib.NewToolResultError("ERROR 1142 (42000): DROP command denied to user 'app'@'localhost' for table 'users'"), nil
	default:
		return mcplib.NewToolResultText("Query OK, 0 rows affected"), nil
	}
}
