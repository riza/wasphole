package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/riza/wasphole/internal/config"
)

// ToolDef is one AI-generated business tool for this deployment.
type ToolDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Params      []string `json:"params"` // parameter names (all strings)
}

// Identity is the AI-generated server persona, cached to disk on first startup.
type Identity struct {
	ServerName  string    `json:"server_name"`
	Description string    `json:"description"`
	Tools       []ToolDef `json:"tools"`
	// Persona is a multi-paragraph system prompt describing the fake company,
	// tech stack, naming conventions, and error message tone. Used as the system
	// prompt for all tool-call AI responses.
	Persona string `json:"persona"`
}

// LoadOrCreate returns the cached Identity from cacheDir/identity.json, or
// generates a new one via the AI client and persists it. Subsequent startups
// with the same cacheDir will always return the cached identity (no API call).
func LoadOrCreate(ctx context.Context, client Client, cacheDir string, instCfg config.InstanceConfig) (*Identity, error) {
	path := filepath.Join(cacheDir, "identity.json")

	// Fast path: load from disk.
	if data, err := os.ReadFile(path); err == nil {
		var id Identity
		if jsonErr := json.Unmarshal(data, &id); jsonErr == nil && id.ServerName != "" {
			return &id, nil
		}
	}

	// Slow path: generate via API.
	id, err := generate(ctx, client, instCfg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("identity: mkdir %s: %w", cacheDir, err)
	}
	data, _ := json.MarshalIndent(id, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, fmt.Errorf("identity: write %s: %w", path, err)
	}
	return id, nil
}

func generate(ctx context.Context, client Client, instCfg config.InstanceConfig) (*Identity, error) {
	mode := instCfg.Mode
	if mode == "" {
		mode = "server"
	}
	industry := instCfg.Industry
	if industry == "" {
		industry = "tech startup"
	}
	seed := instCfg.Seed
	if seed == "" {
		seed = fmt.Sprintf("%016x", time.Now().UnixNano())
	}

	system := `You are helping configure a honeypot MCP server that impersonates a real ` +
		`server to detect malicious AI agents. Generate a convincing fake server identity. ` +
		`Respond using exactly this format and nothing else:

NAME: <short-kebab-case slug, e.g. prod-db-west or api-gateway-02>
DESCRIPTION: <one sentence describing this server's purpose>
TOOL: <tool_name> | <one sentence description> | <param1, param2, ...>
(repeat TOOL line 3-5 times for business-specific tools appropriate for this company — do NOT include generic OS tools like read_file or execute_shell)
PERSONA: <3-5 paragraphs describing the fake company context, tech stack with version numbers, naming conventions, error message tone, and internal jargon — used as a system prompt for AI-generated tool responses>`

	user := fmt.Sprintf("Mode: %s | Industry: %s | Seed: %s", mode, industry, seed)

	raw, err := client.Generate(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("identity: generate: %w", err)
	}

	id, err := parseIdentity(raw)
	if err != nil {
		return nil, fmt.Errorf("identity: %w", err)
	}
	return id, nil
}

func parseIdentity(raw string) (*Identity, error) {
	var id Identity
	// PERSONA may span multiple lines; collect everything after the PERSONA: key.
	inPersona := false
	var personaLines []string

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "NAME:"):
			inPersona = false
			id.ServerName = strings.TrimSpace(strings.TrimPrefix(line, "NAME:"))
		case strings.HasPrefix(line, "DESCRIPTION:"):
			inPersona = false
			id.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		case strings.HasPrefix(line, "TOOL:"):
			inPersona = false
			if t := parseTool(strings.TrimPrefix(line, "TOOL:")); t != nil {
				id.Tools = append(id.Tools, *t)
			}
		case strings.HasPrefix(line, "PERSONA:"):
			inPersona = true
			if rest := strings.TrimSpace(strings.TrimPrefix(line, "PERSONA:")); rest != "" {
				personaLines = append(personaLines, rest)
			}
		case inPersona:
			personaLines = append(personaLines, line)
		}
	}
	id.Persona = strings.TrimSpace(strings.Join(personaLines, "\n"))

	if id.ServerName == "" {
		return nil, fmt.Errorf("missing NAME field in response:\n%s", raw)
	}
	if id.Description == "" {
		return nil, fmt.Errorf("missing DESCRIPTION field in response:\n%s", raw)
	}
	if id.Persona == "" {
		return nil, fmt.Errorf("missing PERSONA field in response:\n%s", raw)
	}
	return &id, nil
}

// parseTool parses "tool_name | description | param1, param2" into a ToolDef.
func parseTool(raw string) *ToolDef {
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) < 2 {
		return nil
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return nil
	}
	t := &ToolDef{
		Name:        name,
		Description: strings.TrimSpace(parts[1]),
	}
	if len(parts) == 3 {
		for _, p := range strings.Split(parts[2], ",") {
			if p = strings.TrimSpace(p); p != "" {
				t.Params = append(t.Params, p)
			}
		}
	}
	return t
}
