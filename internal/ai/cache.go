package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sync/singleflight"
)

// ResponseCache wraps a Client with a lazy disk cache keyed by sha256(tool:params).
// First call to Get generates a response via the AI client and writes it to disk.
// Subsequent identical calls read from disk — no further API calls.
//
// singleflight ensures concurrent cache misses for the same key result in exactly
// one API call; all waiters receive the same response.
type ResponseCache struct {
	client  Client
	dir     string // base cache dir; responses go in dir/responses/
	persona string // system prompt from Identity.Persona
	flight  singleflight.Group
}

// NewResponseCache constructs a ResponseCache. cacheDir is the base directory
// (e.g. "./cache"); persona is the Identity.Persona system prompt.
func NewResponseCache(client Client, cacheDir, persona string) *ResponseCache {
	return &ResponseCache{
		client:  client,
		dir:     cacheDir,
		persona: persona,
	}
}

// Get returns the cached response for (tool, params), generating and caching it
// on first call. description is the tool's human-readable purpose (used in the prompt).
// Concurrent misses for the same key are deduplicated via singleflight.
func (rc *ResponseCache) Get(ctx context.Context, tool, description string, params map[string]any) (string, error) {
	key := cacheKey(tool, params)
	path := filepath.Join(rc.dir, "responses", key+".txt")

	// Fast path: disk hit.
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}

	// Slow path: deduplicate concurrent misses for the same key.
	val, err, _ := rc.flight.Do(key, func() (any, error) {
		// Re-check disk: another goroutine may have written it while we waited.
		if data, err := os.ReadFile(path); err == nil {
			return string(data), nil
		}
		text, err := rc.client.Generate(ctx, rc.persona, toolPrompt(tool, description, params))
		if err != nil {
			return "", fmt.Errorf("cache: generate %s: %w", tool, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", fmt.Errorf("cache: mkdir: %w", err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			return "", fmt.Errorf("cache: write: %w", err)
		}
		return text, nil
	})
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

// cacheKey returns a deterministic sha256 hex key for (tool, params).
// Map key order in Go is randomized; we sort keys before hashing.
func cacheKey(tool string, params map[string]any) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build an ordered key-value list for deterministic JSON marshaling.
	pairs := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, params[k])
	}
	b, _ := json.Marshal(pairs)

	h := sha256.Sum256([]byte(tool + ":" + string(b)))
	return hex.EncodeToString(h[:])
}

// toolPrompt builds the user message for a tool call generation request.
func toolPrompt(tool, description string, params map[string]any) string {
	paramJSON, _ := json.MarshalIndent(params, "", "  ")
	if description == "" {
		description = tool
	}
	return fmt.Sprintf(
		"Simulate the response of this API tool running on the server described in your system context.\n\n"+
			"Tool: %s\n"+
			"Description: %s\n"+
			"Input:\n%s\n\n"+
			"Return a realistic, properly formatted JSON response that this tool would actually return. "+
			"Reference the input values in your response (echo back IDs, show relevant data fields). "+
			"Use field names, error codes, and data formats consistent with the company's tech stack. "+
			"Return only the JSON object — no markdown fences, no explanation.",
		tool, description, paramJSON,
	)
}
