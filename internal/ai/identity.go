package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	system := "You are a configuration generator. Return only valid JSON, no markdown fences."
	user := fmt.Sprintf(
		"Generate a JSON object for a fictional %s. Fields:\n"+
			"- server_name: short kebab-case slug (e.g. \"prod-db-west\", \"api-gateway-02\")\n"+
			"- description: one sentence describing this server's purpose\n"+
			"- persona: 3-5 paragraphs for an AI simulating tool responses from this server.\n"+
			"  Include: company context (industry: %s), tech stack with version numbers,\n"+
			"  naming conventions, error message tone, and internal jargon.\n\n"+
			"Seed for uniqueness: %s\n"+
			"Return only the JSON object.",
		mode, industry, seed,
	)

	raw, err := client.Generate(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("identity: generate: %w", err)
	}

	var id Identity
	if err := json.Unmarshal([]byte(raw), &id); err != nil {
		// Retry once: Claude sometimes wraps in markdown fences despite instructions.
		retryUser := "Return only the JSON object with no markdown fences:\n" + raw
		raw2, err2 := client.Generate(ctx, system, retryUser)
		if err2 != nil {
			return nil, fmt.Errorf("identity: retry generate: %w", err2)
		}
		if err3 := json.Unmarshal([]byte(raw2), &id); err3 != nil {
			return nil, fmt.Errorf("identity: parse failed after retry: %w", err3)
		}
	}

	if id.ServerName == "" {
		return nil, fmt.Errorf("identity: generated JSON missing server_name")
	}
	return &id, nil
}
