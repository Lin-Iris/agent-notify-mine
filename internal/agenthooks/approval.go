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
		HookEventName     string                      `json:"hookEventName"`
		Decision          *hookPermissionRequestReply `json:"decision,omitempty"`
		AdditionalContext string                      `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

type hookPermissionRequestReply struct {
	Behavior     string         `json:"behavior"`
	Reason       string         `json:"reason,omitempty"`
	UpdatedInput map[string]any `json:"updatedInput,omitempty"`
}

const (
	hookPermissionBehaviorAllow = "allow"
	hookPermissionBehaviorDeny  = "deny"
)

var sendApprovalPromptForHook = sendApprovalPrompt

// MaybeHandleApproval 处理远程审批请求。
// 发送飞书审批卡片后阻塞等待用户决策（批准/拒绝）。
// 阻塞时间由 cfg.Approval.TimeoutSeconds 控制（默认 300s）。
// 注意：Claude Code 可能有自身的 hook 超时，如果审批太慢，
// hook 进程可能被 Claude Code 杀掉，导致远程审批失效。
func MaybeHandleApproval(ctx context.Context, cfg config.Config, statePath, logPath string, evt event.Event, stdout io.Writer) (bool, error) {
	if evt.HookEvent != "PermissionRequest" || evt.Status != event.StatusPermissionReq {
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
	_ = state.AppendLog(logPath, fmt.Sprintf("approval created id=%s tool=%s ttl=%v", req.ApprovalID, req.Tool, ttl))
	if err := sendApprovalPromptForHook(ctx, cfg, logPath, evt, req, profileName, profile); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("approval prompt send error id=%s err=%v", req.ApprovalID, err))
	}

	// 阻塞等待飞书审批决策
	_ = state.AppendLog(logPath, fmt.Sprintf("approval waiting id=%s", req.ApprovalID))
	result, err := store.Wait(ctx, req.ApprovalID, ttl)
	if err != nil {
		if writeErr := writeHookDecision(stdout, evt.HookEvent, hookPermissionBehaviorDeny, "approval timed out or failed"); writeErr != nil {
			_ = state.AppendLog(logPath, fmt.Sprintf("approval decision write error id=%s err=%v", req.ApprovalID, writeErr))
		}
		return true, state.AppendLog(logPath, fmt.Sprintf("approval denied by timeout id=%s err=%v", req.ApprovalID, err))
	}
	if result.Status == approval.StatusApproved {
		if writeErr := writeHookDecision(stdout, evt.HookEvent, hookPermissionBehaviorAllow, "approved via Feishu approval "+req.ApprovalID); writeErr != nil {
			_ = state.AppendLog(logPath, fmt.Sprintf("approval decision write error id=%s err=%v", req.ApprovalID, writeErr))
		}
		return true, state.AppendLog(logPath, fmt.Sprintf("approval allowed id=%s operator=%s", req.ApprovalID, result.OperatorID))
	}
	if writeErr := writeHookDecision(stdout, evt.HookEvent, hookPermissionBehaviorDeny, firstNonEmpty(result.Reason, "denied via Feishu approval "+req.ApprovalID)); writeErr != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("approval decision write error id=%s err=%v", req.ApprovalID, writeErr))
	}
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

func writeHookDecision(stdout io.Writer, eventName, behavior, reason string) error {
	if stdout == nil {
		return nil
	}
	var out hookDecisionOutput
	out.HookSpecificOutput.HookEventName = eventName
	out.HookSpecificOutput.Decision = &hookPermissionRequestReply{
		Behavior: behavior,
		Reason:   reason,
	}
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		// stdout 写入失败说明 Claude Code 已经关了管道（hook 超时），
		// 这就是对话框不消失的根本原因。
		fmt.Fprintf(os.Stderr, "FATAL: writeHookDecision encode error: %v\n", err)
		return err
	}
	return nil
}

func writeHookAdditionalContext(stdout io.Writer, eventName, contextText string) error {
	if stdout == nil {
		return nil
	}
	var out hookDecisionOutput
	out.HookSpecificOutput.HookEventName = eventName
	out.HookSpecificOutput.AdditionalContext = contextText
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: writeHookAdditionalContext encode error: %v\n", err)
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
