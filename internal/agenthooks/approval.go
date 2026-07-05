package agenthooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hellolib/agent-notify/internal/approval"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

type hookDecisionOutput struct {
	HookSpecificOutput struct {
		HookEventName            string         `json:"hookEventName"`
		PermissionDecision       string         `json:"permissionDecision"`
		PermissionDecisionReason string         `json:"permissionDecisionReason"`
		UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
		AdditionalContext        string         `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

func MaybeHandleApproval(ctx context.Context, cfg config.Config, statePath, logPath string, evt event.Event, stdout io.Writer) (bool, error) {
	if !cfg.Approval.Enabled || evt.HookEvent != "PermissionRequest" {
		return false, nil
	}
	if evt.Agent == "codex" && cfg.Approval.CodexMode != "hook_decision" {
		return false, state.AppendLog(logPath, "codex approval is notify_only; falling back to normal permission prompt")
	}

	approvalPath, err := config.ApprovalPath()
	if err != nil {
		return true, err
	}
	store := approval.NewStore(approvalPath)
	ttl := time.Duration(cfg.Approval.TimeoutSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	req := approval.NewRequest(evt, ttl)
	if err := store.Create(req); err != nil {
		return true, err
	}
	if err := sendApprovalPrompt(ctx, cfg, statePath, logPath, evt, req); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("approval prompt send error id=%s err=%v", req.ApprovalID, err))
	}

	result, err := store.Wait(ctx, req.ApprovalID, ttl)
	if err != nil {
		writeHookDecision(stdout, evt.HookEvent, "deny", "approval timed out or failed")
		return true, state.AppendLog(logPath, fmt.Sprintf("approval denied by timeout id=%s err=%v", req.ApprovalID, err))
	}
	if result.Status == approval.StatusApproved {
		writeHookDecision(stdout, evt.HookEvent, "allow", "approved via Feishu approval "+req.ApprovalID)
		return true, state.AppendLog(logPath, fmt.Sprintf("approval allowed id=%s operator=%s", req.ApprovalID, result.OperatorID))
	}
	writeHookDecision(stdout, evt.HookEvent, "deny", firstNonEmpty(result.Reason, "denied via Feishu approval "+req.ApprovalID))
	return true, state.AppendLog(logPath, fmt.Sprintf("approval denied id=%s operator=%s", req.ApprovalID, result.OperatorID))
}

func sendApprovalPrompt(ctx context.Context, cfg config.Config, statePath, logPath string, evt event.Event, req approval.Request) error {
	body := fmt.Sprintf("工具: %s\n工作目录: %s\n授权内容:\n%s\n有效期: %s",
		req.Tool, req.Workspace, req.CommandSummary, req.ExpiresAt.Format("15:04:05"))
	msg := notify.Message{
		Agent:         evt.Agent,
		Event:         "permission_required",
		SessionID:     evt.SessionID,
		Workspace:     evt.Workspace,
		Title:         notify.FormatTitle(evt.Agent, "permission_required"),
		Body:          body,
		RawPayload:    string(evt.RawPayload),
		ApprovalID:    req.ApprovalID,
		ApprovalToken: req.Token,
	}
	senders := buildSenders(cfg, msg)
	if len(senders) == 0 {
		return state.AppendLog(logPath, fmt.Sprintf("no sender enabled for approval id=%s", req.ApprovalID))
	}
	dispatcher := notify.NewDispatcher(state.NewStore(statePath), time.Duration(cfg.Behavior.DedupeSeconds)*time.Second, senders...)
	sendCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Behavior.SendTimeoutSeconds)*time.Second)
	defer cancel()
	return dispatcher.SendAll(sendCtx, msg)
}

func writeHookDecision(stdout io.Writer, eventName, decision, reason string) {
	if stdout == nil {
		return
	}
	var out hookDecisionOutput
	out.HookSpecificOutput.HookEventName = eventName
	out.HookSpecificOutput.PermissionDecision = decision
	out.HookSpecificOutput.PermissionDecisionReason = reason
	_ = json.NewEncoder(stdout).Encode(out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
