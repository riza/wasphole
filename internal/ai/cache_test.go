package ai

import (
	"context"
	"testing"
)

type mockClient struct {
	calls int
	resp  string
}

func (m *mockClient) Generate(_ context.Context, _, _ string) (string, error) {
	m.calls++
	return m.resp, nil
}

func TestCacheKey_Deterministic(t *testing.T) {
	params := map[string]any{"path": "/etc/passwd", "mode": "read"}
	k1 := cacheKey("read_file", params)
	k2 := cacheKey("read_file", params)
	if k1 != k2 {
		t.Errorf("cacheKey not deterministic: %q vs %q", k1, k2)
	}
}

func TestCacheKey_MapOrderIndependent(t *testing.T) {
	p1 := map[string]any{"a": "1", "b": "2", "c": "3"}
	p2 := map[string]any{"c": "3", "a": "1", "b": "2"}
	if cacheKey("tool", p1) != cacheKey("tool", p2) {
		t.Error("cacheKey must be map-order independent")
	}
}

func TestCacheKey_DifferentParams(t *testing.T) {
	k1 := cacheKey("read_file", map[string]any{"path": "/etc/passwd"})
	k2 := cacheKey("read_file", map[string]any{"path": "/etc/hosts"})
	if k1 == k2 {
		t.Error("different params must produce different cache keys")
	}
}

func TestCacheKey_DifferentTools(t *testing.T) {
	params := map[string]any{"path": "/etc/passwd"}
	k1 := cacheKey("read_file", params)
	k2 := cacheKey("list_directory", params)
	if k1 == k2 {
		t.Error("different tool names must produce different cache keys")
	}
}

func TestStripMarkdownFences_JSONFence(t *testing.T) {
	got := stripMarkdownFences("```json\n{\"key\": \"val\"}\n```")
	if got != `{"key": "val"}` {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestStripMarkdownFences_GenericFence(t *testing.T) {
	got := stripMarkdownFences("```\nsome content\n```")
	if got != "some content" {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestStripMarkdownFences_NoFence(t *testing.T) {
	in := `{"key": "val"}`
	if got := stripMarkdownFences(in); got != in {
		t.Errorf("no-fence input modified: %q", got)
	}
}

func TestStripMarkdownFences_OnlyOpenFence(t *testing.T) {
	got := stripMarkdownFences("```json\n{\"key\": \"val\"}")
	if got != `{"key": "val"}` {
		t.Errorf("unexpected result for open-only fence: %q", got)
	}
}

func TestResponseCache_HitAfterMiss(t *testing.T) {
	mc := &mockClient{resp: `{"result": "test"}`}
	rc := NewResponseCache(mc, t.TempDir(), "test persona")
	params := map[string]any{"query": "SELECT 1"}

	result1, err := rc.Get(context.Background(), "query_database", "Execute SQL", params)
	if err != nil {
		t.Fatalf("first Get() error: %v", err)
	}
	if mc.calls != 1 {
		t.Fatalf("expected 1 API call on miss, got %d", mc.calls)
	}

	result2, err := rc.Get(context.Background(), "query_database", "Execute SQL", params)
	if err != nil {
		t.Fatalf("second Get() error: %v", err)
	}
	if mc.calls != 1 {
		t.Fatalf("expected no additional API call on hit, got %d total calls", mc.calls)
	}
	if result1 != result2 {
		t.Errorf("cache hit returned different result: %q vs %q", result1, result2)
	}
}

func TestResponseCache_DifferentParamsDifferentKeys(t *testing.T) {
	mc := &mockClient{resp: `{"result": "test"}`}
	rc := NewResponseCache(mc, t.TempDir(), "test persona")

	_, _ = rc.Get(context.Background(), "read_file", "Read a file", map[string]any{"path": "/etc/passwd"})
	_, _ = rc.Get(context.Background(), "read_file", "Read a file", map[string]any{"path": "/etc/hosts"})

	if mc.calls != 2 {
		t.Errorf("expected 2 API calls for 2 different params, got %d", mc.calls)
	}
}
