package codebuddyhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 CodeBuddy hook 传入的 JSON 结构。
type payload struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	Message       string         `json:"message"`
	ToolName      string         `json:"tool_name"`
	ToolResponse  map[string]any `json:"tool_response"`
	ToolInput     map[string]any `json:"tool_input"`
	Matcher       string         `json:"matcher"`
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "codebuddy" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codebuddy",
		HookEvent:   p.HookEventName,
		SessionID:   p.SessionID,
		Workspace:   p.CWD,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch p.HookEventName {
	case "PermissionRequest":
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("codebuddy", "permission_required")
		base.Body = fmt.Sprintf("工具: %s\n操作需要您的授权许可", p.ToolName)
		return base, nil


	case "Stop":
		// Stop 是生命周期信号，不是成功完成。
		// 状态机根据会话上下文推断最终状态。
		base.Status = event.StatusPending
		base.Title = notify.FormatTitle("codebuddy", "running")
		base.Body = notify.DefaultBody("run_completed")
		return base, nil

	case "Notification":
		switch p.Matcher {
		case "permission_prompt":
			base.Status = event.StatusPermissionReq
			base.Title = notify.FormatTitle("codebuddy", "permission_required")
			base.Body = fmt.Sprintf("工具: %s\n操作需要您的授权许可", p.ToolName)
			return base, nil
		case "idle_prompt":
			base.Status = event.StatusInputRequired
			base.Title = notify.FormatTitle("codebuddy", "input_required")
			base.Body = "CodeBuddy 空闲超过 60 秒，正在等待你的输入"
			return base, nil
		default:
			if isInputRequiredMessage(p.Message) {
				base.Status = event.StatusInputRequired
				base.Title = notify.FormatTitle("codebuddy", "input_required")
				base.Body = fmt.Sprintf("提示: %s", extractHint(p.Message))
				return base, nil
			}
			return event.Event{}, fmt.Errorf("unsupported notification matcher: %s", p.Matcher)
		}

	case "PostToolUseFailure":
		base.Status = event.StatusFailed
		base.Title = notify.FormatTitle("codebuddy", "run_failed")
		errMsg := extractErrorMessage(p.ToolResponse)
		base.Body = fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, errMsg)
		return base, nil

	case "SessionEnd":
		// SessionEnd 是会话结束信号，不是成功完成。
		// 状态机根据会话上下文推断最终状态。
		base.Status = event.StatusPending
		base.Title = notify.FormatTitle("codebuddy", "running")
		base.Body = "CodeBuddy 会话已结束"
		return base, nil

	default:
		return event.Event{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

// ── 辅助函数 ──────────────────────────────────────────────

func isInputRequiredMessage(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "waiting for your input") ||
		strings.Contains(msg, "waiting for input") ||
		strings.HasPrefix(msg, "needs input")
}

func extractHint(msg string) string {
	msg = strings.TrimSpace(msg)
	prefixes := []string{
		"codebuddy is waiting for your input",
		"waiting for your input: ",
		"waiting for input: ",
		"needs input: ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(msg), prefix) {
			return strings.TrimSpace(msg[len(prefix):])
		}
	}
	if len(msg) > 100 {
		return msg[:97] + "..."
	}
	return msg
}

func extractErrorMessage(response map[string]any) string {
	if response == nil {
		return "未知错误"
	}
	if err, ok := response["error"]; ok {
		if errStr, ok := err.(string); ok && errStr != "" {
			if len(errStr) > 200 {
				return errStr[:197] + "..."
			}
			return errStr
		}
	}
	if err, ok := response["message"]; ok {
		if errStr, ok := err.(string); ok && errStr != "" {
			if len(errStr) > 200 {
				return errStr[:197] + "..."
			}
			return errStr
		}
	}
	return "操作失败"
}
