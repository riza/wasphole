package mcp

import (
	"context"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/ai"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/sim"
)

func registerResources(s *server.MCPServer, cfg *config.Config, state *sim.SystemState, serverName string, cache *ai.ResponseCache) {
	slug := companySlug(serverName)
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxResources(s, state, slug, cache)
	case "windows":
		registerWindowsResources(s, state, slug, cache)
	}
}

// companySlug extracts a short company identifier from the AI-generated server name.
// "zephyrbay-payments-prod" → "zephyrbay"
func companySlug(serverName string) string {
	if serverName == "" {
		return "corp"
	}
	return strings.SplitN(serverName, "-", 2)[0]
}

func registerLinuxResources(s *server.MCPServer, state *sim.SystemState, slug string, cache *ai.ResponseCache) {
	hostname := state.Hostname
	dbHost := fmt.Sprintf("db01.%s.internal", slug)
	redisHost := fmt.Sprintf("redis.%s.internal", slug)
	dbName := slug + "_production"

	s.AddResource(
		mcplib.NewResource("file:///etc/config.yaml", "Application configuration",
			mcplib.WithMIMEType("text/yaml"),
			mcplib.WithResourceDescription("Main application YAML configuration"),
		),
		func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/yaml",
					Text: fmt.Sprintf("database:\n"+
						"  host: %s\n"+
						"  port: 5432\n"+
						"  name: %s\n"+
						"  pool_size: 20\n"+
						"redis:\n"+
						"  url: redis://%s:6379/0\n"+
						"server:\n"+
						"  host: %s\n"+
						"  port: 8080\n"+
						"log_level: info\n"+
						"feature_flags:\n"+
						"  new_dashboard: true\n"+
						"  beta_api: false\n",
						dbHost, dbName, redisHost, hostname),
				},
			}, nil
		},
	)

	s.AddResource(
		mcplib.NewResource("file:///var/log/app.log", "Application log",
			mcplib.WithMIMEType("text/plain"),
			mcplib.WithResourceDescription("Recent application log entries"),
		),
		func(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			text, err := cache.Get(ctx, "read_app_log",
				"Recent application log file (last 20 lines) showing startup, requests, warnings, and errors",
				map[string]any{"host": hostname, "db_host": dbHost, "db_name": dbName},
			)
			if err != nil {
				text = fmt.Sprintf("ERROR: could not read log: %v\n", err)
			}
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{URI: req.Params.URI, MIMEType: "text/plain", Text: text},
			}, nil
		},
	)

	s.AddResource(
		mcplib.NewResource("file:///app/current/VERSION", "Deployed version file",
			mcplib.WithMIMEType("text/plain"),
			mcplib.WithResourceDescription("Currently deployed application version"),
		),
		func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/plain",
					Text:     "2.14.3\n",
				},
			}, nil
		},
	)
}

func registerWindowsResources(s *server.MCPServer, state *sim.SystemState, slug string, cache *ai.ResponseCache) {
	hostname := state.Hostname
	sqlHost := fmt.Sprintf("sql01.%s.internal", slug)
	dbName := slug + "DB"

	s.AddResource(
		mcplib.NewResource("file:///C:/config/app.config", "Application configuration",
			mcplib.WithMIMEType("text/xml"),
			mcplib.WithResourceDescription("Main application .NET configuration"),
		),
		func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/xml",
					Text: fmt.Sprintf("<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n"+
						"<configuration>\r\n"+
						"  <connectionStrings>\r\n"+
						"    <add name=\"DefaultConnection\" connectionString=\"Data Source=%s;Initial Catalog=%s;Integrated Security=True\" />\r\n"+
						"  </connectionStrings>\r\n"+
						"  <appSettings>\r\n"+
						"    <add key=\"Environment\" value=\"Production\" />\r\n"+
						"    <add key=\"ServerName\" value=\"%s\" />\r\n"+
						"    <add key=\"LogLevel\" value=\"Info\" />\r\n"+
						"    <add key=\"MaxRetries\" value=\"3\" />\r\n"+
						"  </appSettings>\r\n"+
						"</configuration>\r\n",
						sqlHost, dbName, hostname),
				},
			}, nil
		},
	)

	s.AddResource(
		mcplib.NewResource("file:///C:/Logs/app.log", "Application log",
			mcplib.WithMIMEType("text/plain"),
			mcplib.WithResourceDescription("Recent Windows application log entries"),
		),
		func(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			text, err := cache.Get(ctx, "read_app_log_windows",
				"Recent Windows application event log (last 20 entries) showing startup, requests, warnings, and errors",
				map[string]any{"host": hostname, "sql_host": sqlHost, "db_name": dbName},
			)
			if err != nil {
				text = fmt.Sprintf("ERROR: could not read log: %v\r\n", err)
			}
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{URI: req.Params.URI, MIMEType: "text/plain", Text: text},
			}, nil
		},
	)
}
