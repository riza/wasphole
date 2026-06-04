package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testEvent(level Level) Event {
	return Event{
		Level:     level,
		Ts:        time.Now(),
		SessionID: "sess-test",
		Pattern:   "test",
		Message:   "test event message",
	}
}

func TestFormatCEF_Info(t *testing.T) {
	got := formatCEF(testEvent(LevelInfo))
	if !strings.HasPrefix(got, "CEF:0|wasphole|wasphole|1.0|") {
		t.Fatalf("expected CEF prefix, got %q", got)
	}
	if !strings.Contains(got, "|3|") {
		t.Fatalf("expected severity 3 for INFO, got %q", got)
	}
}

func TestFormatCEF_Warn(t *testing.T) {
	got := formatCEF(testEvent(LevelWarn))
	if !strings.Contains(got, "|7|") {
		t.Fatalf("expected severity 7 for WARN, got %q", got)
	}
}

func TestFormatCEF_Critical(t *testing.T) {
	got := formatCEF(testEvent(LevelCritical))
	if !strings.Contains(got, "|10|") {
		t.Fatalf("expected severity 10 for CRITICAL, got %q", got)
	}
}

func TestFormatCEF_ContainsSessionAndPattern(t *testing.T) {
	evt := Event{
		Level:     LevelWarn,
		Ts:        time.Now(),
		SessionID: "sess-xyz",
		Pattern:   "recon",
		Message:   "recon detected",
		Count:     3,
	}
	got := formatCEF(evt)
	if !strings.Contains(got, "sess-xyz") {
		t.Errorf("expected sessionId in CEF ext, got %q", got)
	}
	if !strings.Contains(got, "pattern=recon") {
		t.Errorf("expected pattern in CEF ext, got %q", got)
	}
	if !strings.Contains(got, "cnt=3") {
		t.Errorf("expected cnt in CEF ext, got %q", got)
	}
}

func TestEscapeCEFHeader_Pipe(t *testing.T) {
	got := escapeCEFHeader("foo|bar")
	if strings.Contains(got, "|") && !strings.Contains(got, `\|`) {
		t.Errorf("expected pipe escaped, got %q", got)
	}
	if !strings.Contains(got, `\|`) {
		t.Errorf("expected \\| in output, got %q", got)
	}
}

func TestEscapeCEFHeader_Backslash(t *testing.T) {
	got := escapeCEFHeader(`foo\bar`)
	if !strings.Contains(got, `\\`) {
		t.Errorf("expected backslash escaped, got %q", got)
	}
}

func TestEscapeCEFExt_Equals(t *testing.T) {
	got := escapeCEFExt("key=value")
	if !strings.Contains(got, `\=`) {
		t.Errorf("expected = escaped, got %q", got)
	}
}

func TestEscapeCEFExt_Newline(t *testing.T) {
	got := escapeCEFExt("line1\nline2")
	if strings.Contains(got, "\n") {
		t.Errorf("expected newline escaped, got %q", got)
	}
	if !strings.Contains(got, `\n`) {
		t.Errorf("expected \\n in output, got %q", got)
	}
}

func TestSIEMSink_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/siem.log"
	sink := &SIEMSink{path: path, format: "json"}

	if err := sink.Send(context.Background(), testEvent(LevelWarn)); err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var out Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &out); err != nil {
		t.Fatalf("expected valid JSON line, got %q: %v", data, err)
	}
}

func TestSIEMSink_CEFFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/siem.log"
	sink := &SIEMSink{path: path, format: "cef"}

	if err := sink.Send(context.Background(), testEvent(LevelCritical)); err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "CEF:") {
		t.Fatalf("expected CEF prefix, got %q", data)
	}
}

func TestSIEMSink_MultipleWrites(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/siem.log"
	sink := &SIEMSink{path: path, format: "json"}

	for i := 0; i < 3; i++ {
		if err := sink.Send(context.Background(), testEvent(LevelInfo)); err != nil {
			t.Fatalf("Send() #%d error: %v", i, err)
		}
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(lines))
	}
}

func TestWebhookSink_SendOK(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{url: srv.URL}
	if err := sink.Send(context.Background(), testEvent(LevelWarn)); err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	var evt Event
	if err := json.Unmarshal(received, &evt); err != nil {
		t.Fatalf("webhook received invalid JSON: %v", err)
	}
}

func TestWebhookSink_Send500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink := &WebhookSink{url: srv.URL}
	if err := sink.Send(context.Background(), testEvent(LevelInfo)); err == nil {
		t.Fatal("expected error on 500 response")
	}
}
