package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/config"
)

func registerResources(s *server.MCPServer, cfg *config.Config) {
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxResources(s)
	case "windows":
		registerWindowsResources(s)
	}
}

func registerLinuxResources(s *server.MCPServer) {
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
					Text: "database:\n" +
						"  host: db.corp.internal\n" +
						"  port: 5432\n" +
						"  name: app_production\n" +
						"redis:\n" +
						"  url: redis://127.0.0.1:6379/0\n" +
						"log_level: info\n" +
						"feature_flags:\n" +
						"  new_dashboard: true\n" +
						"  beta_api: false\n",
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
					Text: "2024-01-15T14:22:01Z INFO  Starting application server port=8080\n" +
						"2024-01-15T14:22:02Z INFO  Database connection established host=db.corp.internal\n" +
						"2024-01-15T14:23:45Z WARN  Slow query detected duration=2345ms query=SELECT_users\n" +
						"2024-01-15T14:25:10Z INFO  Request processed path=/api/v2/users status=200\n" +
						"2024-01-15T14:26:03Z ERROR Failed to send email smtp_error=\"connection refused\"\n" +
						"2024-01-15T14:30:00Z INFO  Health check OK uptime=8m\n",
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

func registerWindowsResources(s *server.MCPServer) {
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
					Text: "<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n" +
						"<configuration>\r\n" +
						"  <connectionStrings>\r\n" +
						"    <add name=\"DefaultConnection\" connectionString=\"Data Source=sql01.corp.internal;Initial Catalog=AppDB;Integrated Security=True\" />\r\n" +
						"  </connectionStrings>\r\n" +
						"  <appSettings>\r\n" +
						"    <add key=\"Environment\" value=\"Production\" />\r\n" +
						"    <add key=\"LogLevel\" value=\"Info\" />\r\n" +
						"    <add key=\"MaxRetries\" value=\"3\" />\r\n" +
						"  </appSettings>\r\n" +
						"</configuration>\r\n",
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
					Text: "2024-01-15 14:22:01 [Information] Application started\r\n" +
						"2024-01-15 14:22:03 [Information] Connected to sql01.corp.internal\r\n" +
						"2024-01-15 14:25:10 [Warning] High memory usage: 78%\r\n" +
						"2024-01-15 14:26:03 [Error] SMTP relay unavailable: Connection timed out\r\n" +
						"2024-01-15 14:30:00 [Information] Scheduled task completed: backup\r\n",
				},
			}, nil
		},
	)
}
