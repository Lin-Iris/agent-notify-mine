package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/approval"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
)

func TestBrokerCardActionApprovesPendingRequest(t *testing.T) {
	req := createBrokerApprovalTestRequest(t)

	err := (textApprovalHandler{}).HandleCardAction(context.Background(), "claude-main", "ou_owner", map[string]any{
		"action":      "approve",
		"profile":     "claude-main",
		"approval_id": req.ApprovalID,
		"token":       req.Token,
	})
	if err != nil {
		t.Fatalf("HandleCardAction() error = %v", err)
	}

	got := getApprovalRequest(t, req.ApprovalID)
	if got.Status != approval.StatusApproved {
		t.Fatalf("Status = %s, want %s", got.Status, approval.StatusApproved)
	}
	if got.OperatorID != "ou_owner" {
		t.Fatalf("OperatorID = %q, want ou_owner", got.OperatorID)
	}
}

func TestBrokerCardActionRejectsWrongApprovalToken(t *testing.T) {
	req := createBrokerApprovalTestRequest(t)

	err := (textApprovalHandler{}).HandleCardAction(context.Background(), "claude-main", "ou_owner", map[string]any{
		"action":      "approve",
		"profile":     "claude-main",
		"approval_id": req.ApprovalID,
		"token":       "wrong-token",
	})
	if err == nil {
		t.Fatal("HandleCardAction() error = nil, want token mismatch")
	}

	got := getApprovalRequest(t, req.ApprovalID)
	if got.Status != approval.StatusPending {
		t.Fatalf("Status = %s, want %s", got.Status, approval.StatusPending)
	}
}

func TestEnsureRemoteAgentCLIAvailableChecksClaudePath(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "claude"))
	t.Setenv("PATH", dir)

	if err := ensureRemoteAgentCLIAvailable("claude"); err != nil {
		t.Fatalf("ensureRemoteAgentCLIAvailable() error = %v", err)
	}
}

func TestEnsureRemoteAgentCLIAvailableReturnsClearClaudeError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := ensureRemoteAgentCLIAvailable("claude")
	if err == nil {
		t.Fatal("ensureRemoteAgentCLIAvailable() error = nil, want missing CLI error")
	}
	if !strings.Contains(err.Error(), "Claude CLI") || !strings.Contains(err.Error(), "claude --version") {
		t.Fatalf("error = %q, want actionable Claude CLI message", err.Error())
	}
}

func createBrokerApprovalTestRequest(t *testing.T) approval.Request {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfg.Broker.Enabled = true
	cfg.Profiles["claude-main"] = config.ProfileConfig{
		Agent:   "claude",
		Enabled: true,
		Feishu: config.ProfileFeishuConfig{
			AppID:       "cli_a",
			AppSecret:   "secret",
			OwnerOpenID: "ou_owner",
		},
	}
	cfgPath, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	req := approval.NewRequest(event.Event{
		Agent:     "claude_code",
		HookEvent: "PermissionRequest",
		SessionID: "s1",
		Workspace: filepath.Join(home, "project"),
	}, time.Minute)
	approvalPath, err := config.ApprovalPath()
	if err != nil {
		t.Fatalf("ApprovalPath() error = %v", err)
	}
	if err := approval.NewStore(approvalPath).Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return req
}

func getApprovalRequest(t *testing.T, id string) approval.Request {
	t.Helper()
	approvalPath, err := config.ApprovalPath()
	if err != nil {
		t.Fatalf("ApprovalPath() error = %v", err)
	}
	got, ok, err := approval.NewStore(approvalPath).Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("approval %s not found", id)
	}
	return got
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
