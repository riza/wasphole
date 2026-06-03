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

// Identity is the AI-generated server persona, cached to disk on first startup.
type Identity struct {
	ServerName  string `json:"server_name"`
	Description string `json:"description"`
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
	// PERSONA may span multiple lines; collect them after the PERSONA: key.
	inPersona := false
	var personaLines []string

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "NAME:"):
			id.ServerName = strings.TrimSpace(strings.TrimPrefix(line, "NAME:"))
		case strings.HasPrefix(line, "DESCRIPTION:"):
			id.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			inPersona = false
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
