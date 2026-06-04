package alert

import (
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func makeReadReq(path string) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "read_file",
			Arguments: map[string]any{"path": path},
		},
	}
}

func TestPatternRecon(t *testing.T) {
	s := newSessionState()

	for _, path := range []string{"/etc/hostname", "/proc/cpuinfo"} {
		if _, ok := s.observe("read_file", makeReadReq(path), false); ok {
			t.Errorf("unexpected event on read of %s", path)
		}
	}

	evt, ok := s.observe("read_file", makeReadReq("/var/log/syslog"), false)
	if !ok {
		t.Fatal("expected recon event on 3rd system path read")
	}
	if evt.Pattern != "recon" {
		t.Errorf("expected pattern=recon, got %q", evt.Pattern)
	}
	if evt.Level != LevelWarn {
		t.Errorf("expected LevelWarn, got %q", evt.Level)
	}
}

func TestPatternReconNoRepeat(t *testing.T) {
	s := newSessionState()
	for _, path := range []string{"/etc/hostname", "/proc/cpuinfo", "/var/log/syslog"} {
		s.observe("read_file", makeReadReq(path), false)
	}
	// 4th system path must not fire a second recon alert
	evt, ok := s.observe("read_file", makeReadReq("/etc/os-release"), false)
	if ok && evt.Pattern == "recon" {
		t.Fatal("recon pattern must not fire more than once per session")
	}
}

func TestPatternEscalation(t *testing.T) {
	s := newSessionState()

	// Non-system, non-cred path to avoid triggering other patterns first
	s.observe("read_file", makeReadReq("/home/app/main.go"), false)
	s.observe("write_file", nil, false)

	evt, ok := s.observe("execute_shell", nil, false)
	if !ok {
		t.Fatal("expected escalation event after read+write+execute")
	}
	if evt.Pattern != "escalation" {
		t.Errorf("expected pattern=escalation, got %q", evt.Pattern)
	}
	if evt.Level != LevelCritical {
		t.Errorf("expected LevelCritical, got %q", evt.Level)
	}
}

func TestPatternEscalationNoRepeat(t *testing.T) {
	s := newSessionState()
	s.observe("read_file", makeReadReq("/home/app/main.go"), false)
	s.observe("write_file", nil, false)
	s.observe("execute_shell", nil, false) // first escalation

	evt, ok := s.observe("execute_shell", nil, false)
	if ok && evt.Pattern == "escalation" {
		t.Fatal("escalation must not fire more than once per session")
	}
}

func TestPatternExfil_CredPath(t *testing.T) {
	s := newSessionState()
	evt, ok := s.observe("read_file", makeReadReq("/home/app/.ssh/id_rsa"), false)
	if !ok {
		t.Fatal("expected exfil event for credential path")
	}
	if evt.Pattern != "exfil" {
		t.Errorf("expected pattern=exfil, got %q", evt.Pattern)
	}
}

func TestPatternExfil_BulkReads(t *testing.T) {
	s := newSessionState()
	paths := []string{
		"/home/app/main.go",
		"/home/app/server.go",
		"/home/app/utils.go",
		"/home/app/models.go",
	}
	for _, p := range paths {
		if _, ok := s.observe("read_file", makeReadReq(p), false); ok {
			t.Errorf("unexpected early event on read of %s", p)
		}
	}
	evt, ok := s.observe("read_file", makeReadReq("/home/app/router.go"), false)
	if !ok {
		t.Fatal("expected exfil event after 5 bulk reads")
	}
	if evt.Pattern != "exfil" {
		t.Errorf("expected pattern=exfil, got %q", evt.Pattern)
	}
}

func TestPatternRetry(t *testing.T) {
	s := newSessionState()
	for i := 0; i < 2; i++ {
		if _, ok := s.observe("read_file", nil, true); ok {
			t.Errorf("unexpected event on error %d", i+1)
		}
	}
	evt, ok := s.observe("read_file", nil, true)
	if !ok {
		t.Fatal("expected retry event after 3 consecutive errors")
	}
	if evt.Pattern != "retry" {
		t.Errorf("expected pattern=retry, got %q", evt.Pattern)
	}
}

func TestPatternRetryResetsOnSuccess(t *testing.T) {
	s := newSessionState()
	s.observe("read_file", nil, true)
	s.observe("read_file", nil, true)
	s.observe("read_file", makeReadReq("/home/app/main.go"), false) // success resets counter
	s.observe("read_file", nil, true)
	s.observe("read_file", nil, true)
	// Only 2 errors since last success — should not fire
	if _, ok := s.observe("read_file", nil, true); ok {
		// 3 errors since last reset — this is OK to fire
	}
	// But 2 errors after reset should not fire
	s2 := newSessionState()
	s2.observe("read_file", nil, true)
	s2.observe("read_file", nil, true)
	s2.observe("read_file", makeReadReq("/home/app/x.go"), false) // reset
	_, ok := s2.observe("read_file", nil, true)
	if ok {
		t.Fatal("retry fired after counter was reset by a success")
	}
}

func TestExtractPathParam_FromRequestPath(t *testing.T) {
	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "read_file",
			Arguments: map[string]any{"path": "/etc/passwd"},
		},
	}
	if got := extractPathParam(req); got != "/etc/passwd" {
		t.Errorf("extractPathParam() = %q, want /etc/passwd", got)
	}
}

func TestExtractPathParam_FromRequestKey(t *testing.T) {
	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "read_registry",
			Arguments: map[string]any{"key": `HKEY_LOCAL_MACHINE\SOFTWARE`},
		},
	}
	if got := extractPathParam(req); got != `HKEY_LOCAL_MACHINE\SOFTWARE` {
		t.Errorf("extractPathParam() = %q", got)
	}
}

func TestExtractPathParam_FromPointer(t *testing.T) {
	req := &mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "read_file",
			Arguments: map[string]any{"path": "/var/log/app.log"},
		},
	}
	if got := extractPathParam(req); got != "/var/log/app.log" {
		t.Errorf("extractPathParam(*req) = %q", got)
	}
}

func TestExtractPathParam_NilPointer(t *testing.T) {
	var req *mcplib.CallToolRequest
	if got := extractPathParam(req); got != "" {
		t.Errorf("extractPathParam(nil ptr) = %q, want empty", got)
	}
}

func TestExtractPathParam_Nil(t *testing.T) {
	if got := extractPathParam(nil); got != "" {
		t.Errorf("extractPathParam(nil) = %q, want empty", got)
	}
}
