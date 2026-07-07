package claudehooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/event"
)

var testAdapter = Adapter{}

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "permission_request.json"))
	if err != nil {
		t.Fatal(err)
	}

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Status != event.StatusPermissionReq {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusPermissionReq)
	}
	if evt.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", evt.Agent)
	}
	if evt.SpecVersion != event.CurrentSpecVersion {
		t.Fatalf("SpecVersion = %q, want %q", evt.SpecVersion, event.CurrentSpecVersion)
	}
	if evt.EventID == "" {
		t.Fatal("EventID should not be empty")
	}
	if len(evt.RawPayload) == 0 {
		t.Fatal("RawPayload should not be empty")
	}
	if !strings.Contains(evt.Body, "git status") {
		t.Fatalf("Body = %q, want authorization content", evt.Body)
	}
}

func TestParseAskUserQuestionPermissionRequestAsInputRequired(t *testing.T) {
	data := []byte(`{
		"hook_event_name":"PermissionRequest",
		"session_id":"s1",
		"cwd":"/tmp/project",
		"tool_name":"AskUserQuestion",
		"tool_input":{
			"questions":[{
				"question":"看到这个等待用户选择的弹窗了吗？",
				"header":"等待输入测试",
				"multiSelect":false,
				"options":[
					{"label":"看到了","description":"弹窗显示了，没问题"},
					{"label":"没看到","description":"弹窗没出来"}
				]
			}]
		}
	}`)

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Status != event.StatusInputRequired {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusInputRequired)
	}
	if !strings.Contains(evt.Body, "看到这个等待用户选择的弹窗了吗？") {
		t.Fatalf("Body = %q, want question text", evt.Body)
	}
}

func TestParseNotificationWaitingInput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "notification_waiting_input.json"))
	if err != nil {
		t.Fatal(err)
	}

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Status != event.StatusInputRequired {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusInputRequired)
	}
	if evt.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", evt.Agent)
	}
	if evt.Body != "提示: " {
		t.Fatalf("Body = %q, want %q", evt.Body, "提示: ")
	}
}

func TestParseNotificationNeedsInputVariant(t *testing.T) {
	data := []byte(`{"hook_event_name":"Notification","session_id":"s1","cwd":"/tmp/project","message":"needs input: please confirm"}`)

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Status != event.StatusInputRequired {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusInputRequired)
	}
	if evt.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", evt.Agent)
	}
	if evt.Body != "提示: please confirm" {
		t.Fatalf("Body = %q, want %q", evt.Body, "提示: please confirm")
	}
}

func TestParseStopReturnsCompleted(t *testing.T) {
	// Stop 是任务完成提醒，adapter 直接映射成 completed。
	data := []byte(`{"hook_event_name":"Stop","session_id":"s1","cwd":"/tmp"}`)

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.HookEvent != "Stop" {
		t.Fatalf("HookEvent = %q, want Stop", evt.HookEvent)
	}
	if evt.Status != event.StatusCompleted {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusCompleted)
	}
	if evt.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", evt.Agent)
	}
}

func TestParsePostToolUseFailure(t *testing.T) {
	data := []byte(`{"hook_event_name":"PostToolUseFailure","session_id":"s1","cwd":"/tmp","tool_name":"Bash","tool_response":{"error":"command not found"}}`)

	evt, err := testAdapter.Parse(data)
	if err != nil {
		t.Fatalf("Adapter.Parse() error = %v", err)
	}
	if evt.Status != event.StatusFailed {
		t.Fatalf("Status = %q, want %q", evt.Status, event.StatusFailed)
	}
	if evt.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", evt.Agent)
	}
}
