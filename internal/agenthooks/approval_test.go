package agenthooks

import (
	"bytes"
	"context"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
)

func TestMaybeHandleApprovalSkipsPlainNotificationHooks(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "")
	cfg := config.Default()
	cfg.Approval.Enabled = true
	cfg.Broker.Enabled = true

	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", event.Event{
		Agent:     "codex",
		HookEvent: "PermissionRequest",
		Status:    event.StatusPermissionReq,
		SessionID: "s1",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if handled {
		t.Fatal("plain notification hook should not enter remote approval flow")
	}
}

func TestMaybeHandleApprovalSkipsDisabledRemoteProfile(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_REMOTE_PROFILE", "codex-main")
	cfg := config.Default()
	cfg.Approval.Enabled = true
	cfg.Broker.Enabled = true
	profile := cfg.Profiles["codex-main"]
	profile.Enabled = false
	cfg.Profiles["codex-main"] = profile

	handled, err := MaybeHandleApproval(context.Background(), cfg, t.TempDir()+"/state.json", t.TempDir()+"/log.txt", event.Event{
		Agent:     "codex",
		HookEvent: "PermissionRequest",
		Status:    event.StatusPermissionReq,
		SessionID: "s1",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("MaybeHandleApproval() error = %v", err)
	}
	if handled {
		t.Fatal("disabled remote profile should not enter approval flow")
	}
}
