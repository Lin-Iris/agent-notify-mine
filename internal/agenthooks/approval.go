package agenthooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	if evt.HookEvent != "PermissionRequest" {
		return false, nil
	}

	// Determine profile for approval
	profileName := os.Getenv("AGENT_NOTIFY_REMOTE_PROFILE")
	if profileName == "" {
		profileName = defaultProfileForAgent(evt.Agent)
	}
	if profileName == "" {
		return false, state.AppendLog(logPath, fmt.Sprintf("approval not available: no profile for agent %s", evt.Agent))
	}

	if !cfg.Broker.Enabled || !cfg.Approval.Enabled {
		return false, state.AppendLog(logPath, "remote approval skipped: broker or approval disabled")
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok || !profile.Enabled {
		return false, state.AppendLog(logPath, fmt.Sprintf("approval skipped: profile %s disabled or missing", profileName))
	}
	if !profile.Feishu.HasCredentials() {
		return false, state.AppendLog(logPath, fmt.Sprintf("approval skipped: profile %s has no feishu credentials", profileName))
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
	if err := sendApprovalPrompt(ctx, cfg, logPath, evt, req, profileName, profile); err != nil {
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

func sendApprovalPrompt(ctx context.Context, cfg config.Config, logPath string, evt event.Event, req approval.Request, profileName string, profile config.ProfileConfig) error {
	body := fmt.Sprintf("工具: %s\n工作目录: %s\n授权内容:\n%s\n有效期: %s",
		req.Tool, req.Workspace, req.CommandSummary, req.ExpiresAt.Format("15:04:05"))
	msg := notify.Message{
		Agent:         evt.Agent,
		Event:         "permission_required",
		SessionID:     evt.SessionID,
		Workspace:     evt.Workspace,
		Profile:       profileName,
		Title:         notify.FormatTitle(evt.Agent, "permission_required"),
		Body:          body,
		RawPayload:    string(evt.RawPayload),
		ApprovalID:    req.ApprovalID,
		ApprovalToken: req.Token,
	}

	// Always send approval via profile feishu (not affected by channels.feishu.enabled)
	sender, err := notify.NewProfileFeishuSender(profileName, profile)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("approval feishu sender error: %v", err))
	}
	sendCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Behavior.SendTimeoutSeconds)*time.Second)
	defer cancel()
	return sender.Send(sendCtx, msg)
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
