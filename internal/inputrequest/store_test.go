package inputrequest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
)

func TestNewRequestExtractsOptions(t *testing.T) {
	req := NewRequest(testInputEvent("needs input: 能看到这个弹窗吗？\n1. 看到了\n2. 没看到\nOther"), "claude-main", time.Minute)

	if req.Prompt != "能看到这个弹窗吗？" {
		t.Fatalf("Prompt = %q", req.Prompt)
	}
	want := []string{"看到了", "没看到", "Other"}
	if len(req.Options) != len(want) {
		t.Fatalf("Options = %#v, want %#v", req.Options, want)
	}
	for i := range want {
		if req.Options[i] != want[i] {
			t.Fatalf("Options = %#v, want %#v", req.Options, want)
		}
	}
	if !req.AllowOther {
		t.Fatal("AllowOther should be true")
	}
}

func TestNewRequestExtractsAskUserQuestion(t *testing.T) {
	req := NewRequest(testAskUserQuestionEvent(true), "claude-main", time.Minute)

	if req.Prompt != "看到这个等待用户选择的弹窗了吗？" {
		t.Fatalf("Prompt = %q", req.Prompt)
	}
	if !req.MultiSelect {
		t.Fatal("MultiSelect should be true")
	}
	want := []string{"看到了", "没看到", "Other"}
	if len(req.Options) != len(want) {
		t.Fatalf("Options = %#v, want %#v", req.Options, want)
	}
	for i := range want {
		if req.Options[i] != want[i] {
			t.Fatalf("Options = %#v, want %#v", req.Options, want)
		}
	}
	if !req.AllowOther {
		t.Fatal("AllowOther should be true")
	}
}

func TestStoreAnswerValuesStoresMultiSelectAndOther(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inputs.json"))
	req := NewRequest(testAskUserQuestionEvent(true), "claude-main", time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.AnswerValues(req.InputID, req.Token, "ou_user", []string{"看到了", "没看到"}, "补充说明")
	if err != nil {
		t.Fatalf("AnswerValues() error = %v", err)
	}
	if got.Answer != "看到了, 没看到, 补充说明" {
		t.Fatalf("Answer = %q", got.Answer)
	}
	if got.OtherAnswer != "补充说明" {
		t.Fatalf("OtherAnswer = %q", got.OtherAnswer)
	}
}

func TestStoreAnswerPendingRequest(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inputs.json"))
	req := NewRequest(testInputEvent("waiting for input"), "claude-main", time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.Answer(req.InputID, req.Token, "ou_user", "看到了")
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if got.Status != StatusAnswered {
		t.Fatalf("Status = %s, want %s", got.Status, StatusAnswered)
	}
	if got.Answer != "看到了" {
		t.Fatalf("Answer = %q", got.Answer)
	}
}

func TestStorePendingForProfileReturnsOldest(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inputs.json"))
	req1 := NewRequest(testInputEvent("first"), "claude-main", time.Minute)
	req2 := NewRequest(testInputEvent("second"), "codex-main", time.Minute)
	if err := store.Create(req1); err != nil {
		t.Fatalf("Create(req1) error = %v", err)
	}
	if err := store.Create(req2); err != nil {
		t.Fatalf("Create(req2) error = %v", err)
	}

	got, ok, err := store.PendingForProfile("claude-main")
	if err != nil || !ok {
		t.Fatalf("PendingForProfile() ok=%v err=%v", ok, err)
	}
	if got.InputID != req1.InputID {
		t.Fatalf("InputID = %q, want %q", got.InputID, req1.InputID)
	}
}

func TestStoreWaitReceivesAnswer(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inputs.json"))
	req := NewRequest(testInputEvent("waiting"), "claude-main", time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = store.Answer(req.InputID, req.Token, "ou_user", "ok")
	}()
	got, err := store.Wait(context.Background(), req.InputID, time.Second)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if got.Answer != "ok" {
		t.Fatalf("Answer = %q, want ok", got.Answer)
	}
}

func testInputEvent(message string) event.Event {
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "Notification",
		"message":         message,
	})
	return event.Event{
		Agent:      "claude_code",
		HookEvent:  "Notification",
		Status:     event.StatusInputRequired,
		SessionID:  "s1",
		Workspace:  "/tmp/project",
		RawPayload: raw,
		Body:       "提示: " + message,
	}
}

func testAskUserQuestionEvent(multi bool) event.Event {
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PermissionRequest",
		"tool_name":       "AskUserQuestion",
		"tool_input": map[string]any{
			"questions": []any{
				map[string]any{
					"question":    "看到这个等待用户选择的弹窗了吗？",
					"header":      "等待输入测试",
					"multiSelect": multi,
					"options": []any{
						map[string]any{"label": "看到了", "description": "弹窗显示了，没问题"},
						map[string]any{"label": "没看到", "description": "弹窗没出来"},
						map[string]any{"label": "Other"},
					},
				},
			},
		},
	})
	return event.Event{
		Agent:      "claude_code",
		HookEvent:  "PermissionRequest",
		Status:     event.StatusInputRequired,
		SessionID:  "s1",
		Workspace:  "/tmp/project",
		RawPayload: raw,
	}
}
