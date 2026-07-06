package cursorhooks

import (
	"testing"

	"github.com/hellolib/agent-notify/internal/event"
)

func TestParseCompletedStop(t *testing.T) {
	raw := []byte(`{"hook_event_name":"stop","conversation_id":"c1","workspace_roots":["/tmp/project"],"status":"completed"}`)

	evt, err := defaultAdapter.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evt.Agent != "cursor" {
		t.Fatalf("Agent = %q, want cursor", evt.Agent)
	}
	if evt.Status != event.StatusCompleted {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusCompleted)
	}
	if evt.Workspace != "/tmp/project" {
		t.Fatalf("Workspace = %q, want /tmp/project", evt.Workspace)
	}
}
