package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/sim"
)

func registerResources(s *server.MCPServer, cfg *config.Config, state *sim.SystemState, serverName string) {
	slug := companySlug(serverName)
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxResources(s, state, slug)
	case "windows":
		registerWindowsResources(s, state, slug)
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

func registerLinuxResources(s *server.MCPServer, state *sim.SystemState, slug string) {
	hostname := state.Hostname
	dbHost := fmt.Sprintf("db01.%s.internal", slug)
	redisHost := fmt.Sprintf("redis.%s.internal", slug)
	dbName := slug + "_production"
	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")

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
		func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/plain",
					Text: fmt.Sprintf(
						"%s INFO  Starting application server host=%s port=8080\n"+
							"%s INFO  Database connection established host=%s db=%s\n"+
							"%s WARN  Slow query detected duration=2345ms query=SELECT_orders\n"+
							"%s INFO  Request processed path=/api/v2/orders status=200\n"+
							"%s ERROR Failed to send notification smtp_error=\"connection refused\"\n"+
							"%s INFO  Health check OK uptime=8m\n",
						ts, hostname,
						ts, dbHost, dbName,
						ts,
						ts,
						ts,
						ts,
					),
				},
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

func registerWindowsResources(s *server.MCPServer, state *sim.SystemState, slug string) {
	hostname := state.Hostname
	sqlHost := fmt.Sprintf("sql01.%s.internal", slug)
	dbName := slug + "DB"
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")

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
		func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/plain",
					Text: fmt.Sprintf(
						"%s [Information] Application started on %s\r\n"+
							"%s [Information] Connected to %s database=%s\r\n"+
							"%s [Warning] High memory usage: 78%%\r\n"+
							"%s [Error] SMTP relay unavailable: Connection timed out\r\n"+
							"%s [Information] Scheduled task completed: backup\r\n",
						ts, hostname,
						ts, sqlHost, dbName,
						ts,
						ts,
						ts,
					),
				},
			}, nil
		},
	)
}
