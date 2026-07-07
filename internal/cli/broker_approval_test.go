package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/approval"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/inputrequest"
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

func TestBrokerCardActionAnswersInputRequest(t *testing.T) {
	req := createBrokerInputTestRequest(t)

	err := (textApprovalHandler{}).HandleCardAction(context.Background(), "claude-main", "ou_owner", map[string]any{
		"action":   "input_submit",
		"profile":  "claude-main",
		"input_id": req.InputID,
		"token":    req.Token,
		"_form_value": map[string]any{
			"answer": "看到了",
		},
	})
	if err != nil {
		t.Fatalf("HandleCardAction() error = %v", err)
	}

	got := getInputRequest(t, req.InputID)
	if got.Status != inputrequest.StatusAnswered {
		t.Fatalf("Status = %s, want %s", got.Status, inputrequest.StatusAnswered)
	}
	if got.Answer != "看到了" {
		t.Fatalf("Answer = %q", got.Answer)
	}
}

func TestBrokerCardActionAnswersInputRequestWithMultiSelectAndOther(t *testing.T) {
	req := createBrokerInputTestRequest(t)

	err := (textApprovalHandler{}).HandleCardAction(context.Background(), "claude-main", "ou_owner", map[string]any{
		"action":   "input_submit",
		"profile":  "claude-main",
		"input_id": req.InputID,
		"token":    req.Token,
		"_form_value": map[string]any{
			"answer": []any{"看到了", "没看到"},
			"other":  "补充说明",
		},
	})
	if err != nil {
		t.Fatalf("HandleCardAction() error = %v", err)
	}

	got := getInputRequest(t, req.InputID)
	if got.Answer != "看到了, 没看到, 补充说明" {
		t.Fatalf("Answer = %q", got.Answer)
	}
}

func TestBrokerTextAnswersPendingInputBeforeStartingTask(t *testing.T) {
	req := createBrokerInputTestRequest(t)

	err := (textApprovalHandler{}).HandleText(context.Background(), "claude-main", "", "ou_owner", "自定义答案")
	if err != nil {
		t.Fatalf("HandleText() error = %v", err)
	}

	got := getInputRequest(t, req.InputID)
	if got.Status != inputrequest.StatusAnswered {
		t.Fatalf("Status = %s, want %s", got.Status, inputrequest.StatusAnswered)
	}
	if got.Answer != "自定义答案" {
		t.Fatalf("Answer = %q", got.Answer)
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

func createBrokerInputTestRequest(t *testing.T) inputrequest.Request {
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
	raw, _ := json.Marshal(map[string]any{
		"hook_event_name": "PermissionRequest",
		"tool_name":       "AskUserQuestion",
		"tool_input": map[string]any{
			"questions": []any{
				map[string]any{
					"question":    "看到这个等待用户选择的弹窗了吗？",
					"multiSelect": true,
					"options": []any{
						map[string]any{"label": "看到了"},
						map[string]any{"label": "没看到"},
						map[string]any{"label": "Other"},
					},
				},
			},
		},
	})
	req := inputrequest.NewRequest(event.Event{
		Agent:      "claude_code",
		HookEvent:  "PermissionRequest",
		Status:     event.StatusInputRequired,
		SessionID:  "s1",
		Workspace:  filepath.Join(home, "project"),
		RawPayload: raw,
	}, "claude-main", time.Minute)
	inputPath, err := config.InputRequestsPath()
	if err != nil {
		t.Fatalf("InputRequestsPath() error = %v", err)
	}
	if err := inputrequest.NewStore(inputPath).Create(req); err != nil {
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

func getInputRequest(t *testing.T, id string) inputrequest.Request {
	t.Helper()
	inputPath, err := config.InputRequestsPath()
	if err != nil {
		t.Fatalf("InputRequestsPath() error = %v", err)
	}
	got, ok, err := inputrequest.NewStore(inputPath).Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("input request %s not found", id)
	}
	return got
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
