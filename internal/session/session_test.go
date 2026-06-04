package session

import (
	"fmt"
	"testing"
	"time"
)

func TestNullRecorder_NoOp(t *testing.T) {
	r := NullRecorder{}
	r.RecordRequest("s1", "read_file", nil)
	r.RecordResponse("s1", "read_file", "result")
	r.RecordError("s1", "read_file", nil)
}

func TestFileRecorder_WriteAndExport(t *testing.T) {
	dir := t.TempDir()
	rec, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatalf("NewFileRecorder() error: %v", err)
	}
	defer rec.Close()

	rec.RecordRequest("sess-1", "read_file", map[string]any{"path": "/etc/passwd"})
	rec.RecordResponse("sess-1", "read_file", "root:x:0:0:...")
	rec.RecordError("sess-1", "write_file", fmt.Errorf("permission denied"))

	sess, err := Export(dir, "sess-1")
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if len(sess.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sess.Entries))
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", sess.SessionID)
	}

	types := map[string]bool{}
	for _, e := range sess.Entries {
		types[e.Type] = true
		if e.SessionID != "sess-1" {
			t.Errorf("entry SessionID = %q, want sess-1", e.SessionID)
		}
	}
	for _, want := range []string{"request", "response", "error"} {
		if !types[want] {
			t.Errorf("missing entry type %q", want)
		}
	}
}

func TestFileRecorder_MethodsPreserved(t *testing.T) {
	dir := t.TempDir()
	rec, _ := NewFileRecorder(dir)
	defer rec.Close()

	rec.RecordRequest("sess-m", "execute_shell", nil)
	rec.RecordResponse("sess-m", "execute_shell", "uid=0(root)")

	sess, _ := Export(dir, "sess-m")
	for _, e := range sess.Entries {
		if e.Method != "execute_shell" {
			t.Errorf("Method = %q, want execute_shell", e.Method)
		}
	}
}

func TestExport_SessionNotFound(t *testing.T) {
	_, err := Export(t.TempDir(), "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestExportReplay_DeltaTiming(t *testing.T) {
	dir := t.TempDir()
	rec, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}

	rec.RecordRequest("sess-r", "read_file", nil)
	time.Sleep(15 * time.Millisecond)
	rec.RecordResponse("sess-r", "read_file", "result")
	rec.Close()

	rs, err := ExportReplay(dir, "sess-r")
	if err != nil {
		t.Fatalf("ExportReplay() error: %v", err)
	}
	if rs.SessionID != "sess-r" {
		t.Errorf("SessionID = %q, want sess-r", rs.SessionID)
	}
	if len(rs.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(rs.Entries))
	}
	if rs.Entries[0].DeltaMs != 0 {
		t.Errorf("first entry DeltaMs = %d, want 0", rs.Entries[0].DeltaMs)
	}
	if rs.Entries[1].DeltaMs <= 0 {
		t.Errorf("second entry DeltaMs = %d, want > 0", rs.Entries[1].DeltaMs)
	}
	if rs.DurMs != rs.Entries[1].DeltaMs {
		t.Errorf("DurMs = %d, want %d (last entry delta)", rs.DurMs, rs.Entries[1].DeltaMs)
	}
}

func TestExportReplay_SingleEntry(t *testing.T) {
	dir := t.TempDir()
	rec, _ := NewFileRecorder(dir)
	rec.RecordRequest("sess-s", "read_file", nil)
	rec.Close()

	rs, err := ExportReplay(dir, "sess-s")
	if err != nil {
		t.Fatalf("ExportReplay() error: %v", err)
	}
	if rs.DurMs != 0 {
		t.Errorf("single-entry session DurMs = %d, want 0", rs.DurMs)
	}
}
