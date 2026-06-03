package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

// StdoutSink writes JSON-encoded events to stdout.
type StdoutSink struct{}

func (s *StdoutSink) Send(_ context.Context, evt Event) error {
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(b))
	return nil
}

// WebhookSink POSTs JSON-encoded events to a configured URL with optional HMAC signing.
type WebhookSink struct {
	url    string
	secret string // base64-encoded webhook secret; empty = no signing
}

func (s *WebhookSink) Send(ctx context.Context, evt Event) error {
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if s.secret != "" {
		wh, err := standardwebhooks.NewWebhook(s.secret)
		if err == nil {
			msgID := uuid.NewString()
			ts := time.Now()
			sig, err := wh.Sign(msgID, ts, b)
			if err == nil {
				req.Header.Set(standardwebhooks.HeaderWebhookID, msgID)
				req.Header.Set(standardwebhooks.HeaderWebhookSignature, sig)
				req.Header.Set(standardwebhooks.HeaderWebhookTimestamp, fmt.Sprint(ts.Unix()))
			}
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// SIEMSink writes CEF or JSON formatted events to a file or stderr.
type SIEMSink struct {
	path   string // output file path; empty = os.Stderr
	format string // "cef" (default) or "json"
	mu     sync.Mutex
	file   *os.File // lazy-opened on first write
}

func (s *SIEMSink) Send(_ context.Context, evt Event) error {
	var line string
	if s.format == "json" {
		b, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		line = string(b)
	} else {
		line = formatCEF(evt)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := fmt.Fprintln(s.writer(), line)
	return err
}

func (s *SIEMSink) writer() io.Writer {
	if s.path == "" {
		return os.Stderr
	}
	if s.file == nil {
		f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return os.Stderr
		}
		s.file = f
	}
	return s.file
}

func formatCEF(evt Event) string {
	severity := "3"
	switch evt.Level {
	case LevelWarn:
		severity = "7"
	case LevelCritical:
		severity = "10"
	}
	classID := escapeCEFHeader(string(evt.Level))
	name := escapeCEFHeader(evt.Message)
	ext := fmt.Sprintf("sessionId=%s msg=%s",
		escapeCEFExt(evt.SessionID),
		escapeCEFExt(evt.Message))
	if evt.Pattern != "" {
		ext += " pattern=" + escapeCEFExt(evt.Pattern)
	}
	if evt.Count > 0 {
		ext += fmt.Sprintf(" cnt=%d", evt.Count)
	}
	return fmt.Sprintf("CEF:0|wasphole|wasphole|1.0|%s|%s|%s|%s",
		classID, name, severity, ext)
}

func escapeCEFHeader(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `|`, `\|`)
}

func escapeCEFExt(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	return strings.ReplaceAll(s, "\n", `\n`)
}
