package hermeshooks

import (
	"testing"

	"github.com/hellolib/agent-notify/internal/event"
)

func TestParsePostLLMCallCompleted(t *testing.T) {
	raw := []byte(`{"hook_event_name":"post_llm_call","session_id":"h1","cwd":"/tmp/project","extra":{"user_message":"run tests"}}`)

	evt, err := defaultAdapter.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if evt.Agent != "hermes" {
		t.Fatalf("Agent = %q, want hermes", evt.Agent)
	}
	if evt.Status != event.StatusCompleted {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusCompleted)
	}
	if evt.Workspace != "/tmp/project" {
		t.Fatalf("Workspace = %q, want /tmp/project", evt.Workspace)
	}
}
