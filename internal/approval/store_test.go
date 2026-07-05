package approval

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
)

func TestStoreDecideApprovesPendingRequest(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "approvals.json"))
	req := NewRequest(testEvent(), time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.Decide(req.ApprovalID, req.Token, "ou_user", DecisionApprove, "ok")
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if got.Status != StatusApproved {
		t.Fatalf("Status = %s, want %s", got.Status, StatusApproved)
	}
	if got.OperatorID != "ou_user" {
		t.Fatalf("OperatorID = %q, want ou_user", got.OperatorID)
	}
	if got.DecidedAt == nil {
		t.Fatal("DecidedAt should be set")
	}
}

func TestStoreRejectsWrongToken(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "approvals.json"))
	req := NewRequest(testEvent(), time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := store.Decide(req.ApprovalID, "wrong", "ou_user", DecisionApprove, "ok"); err == nil {
		t.Fatal("Decide() error = nil, want token mismatch")
	}
	got, ok, err := store.Get(req.ApprovalID)
	if err != nil || !ok {
		t.Fatalf("Get() = ok:%v err:%v", ok, err)
	}
	if got.Status != StatusPending {
		t.Fatalf("Status = %s, want %s", got.Status, StatusPending)
	}
}

func TestStoreRejectsSecondDecision(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "approvals.json"))
	req := NewRequest(testEvent(), time.Minute)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Decide(req.ApprovalID, req.Token, "ou_user", DecisionDeny, "no"); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if _, err := store.Decide(req.ApprovalID, req.Token, "ou_user", DecisionApprove, "ok"); err == nil {
		t.Fatal("second Decide() error = nil, want already decided")
	}
}

func TestStoreWaitTimesOut(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "approvals.json"))
	req := NewRequest(testEvent(), 10*time.Millisecond)
	if err := store.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := store.Wait(context.Background(), req.ApprovalID, 20*time.Millisecond); err == nil {
		t.Fatal("Wait() error = nil, want timeout")
	}
}

func TestNewRequestStoresReadableCommandSummary(t *testing.T) {
	req := NewRequest(testEvent(), time.Minute)

	if req.CommandSummary != "git status" {
		t.Fatalf("CommandSummary = %q, want git status", req.CommandSummary)
	}
	if req.CommandDigest == "" {
		t.Fatal("CommandDigest should still be populated")
	}
}

func testEvent() event.Event {
	raw, _ := json.Marshal(map[string]any{
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git status",
		},
	})
	return event.Event{
		Agent:      "claude_code",
		HookEvent:  "PermissionRequest",
		Status:     event.StatusPermissionReq,
		SessionID:  "s1",
		Workspace:  "/tmp/project",
		RawPayload: raw,
	}
}
