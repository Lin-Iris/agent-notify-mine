package codexhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/event"
)

var testAdapter = Adapter{}

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex-hooks", "permission_request.json"))
	if err != nil {
		t.Fatal(err)
	}

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", evt.Agent)
	}
	if evt.Status != event.StatusPermissionReq {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusPermissionReq)
	}
	if !strings.Contains(evt.Body, "Bash") {
		t.Fatalf("Body = %q, want tool name Bash", evt.Body)
	}
	if evt.Workspace != "/tmp/demo" {
		t.Fatalf("Workspace = %q, want /tmp/demo", evt.Workspace)
	}
}

func TestParseStopReturnsPending(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", evt.Agent)
	}
	if evt.HookEvent != "Stop" {
		t.Fatalf("HookEvent = %q, want Stop", evt.HookEvent)
	}
	if evt.Status != event.StatusPending {
		t.Fatalf("Status = %q, want %q (Stop should not map to completed)", evt.Status, event.StatusPending)
	}
	// last_assistant_message 非空时应作为 Body
	if !strings.Contains(evt.Body, "cargo build") {
		t.Fatalf("Body = %q, want last_assistant_message content", evt.Body)
	}
}

func TestParseStopFallsBackToDefaultBody(t *testing.T) {
	raw := []byte(`{"hook_event_name":"Stop","session_id":"s","cwd":"/tmp","last_assistant_message":""}`)

	evt, err := testAdapter.Parse(raw)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Body == "" {
		t.Fatal("Body should fall back to default when last_assistant_message empty")
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	raw := []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s","cwd":"/tmp"}`)

	_, err := testAdapter.Parse(raw)
	if err == nil {
		t.Fatal("Adapter.Parse() expected error for unsupported event")
	}
}

func TestParseLegacyCamelCaseEvents(t *testing.T) {
	for _, raw := range []string{
		`{"hook_event_name":"Stop","session_id":"s"}`,
		`{"hook_event_name":"PermissionRequest","session_id":"s"}`,
	} {
		if _, err := testAdapter.Parse([]byte(raw)); err != nil {
			t.Fatalf("legacy payload %s should remain supported: %v", raw, err)
		}
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		in    string
		limit int
		want  string
	}{
		{"", 10, ""},
		{"short", 10, "short"},
		{"1234567890ab", 10, "1234567..."},
	}
	for _, tt := range tests {
		got := truncateMessage(tt.in, tt.limit)
		if got != tt.want {
			t.Fatalf("truncateMessage(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
		}
	}
}
