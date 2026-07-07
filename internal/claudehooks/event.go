package claudehooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Claude Code 通过 stdin 投递的 hook JSON。
type payload struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	Message       string         `json:"message"`
	ToolName      string         `json:"tool_name"`
	ToolResponse  map[string]any `json:"tool_response"`
	ToolInput     map[string]any `json:"tool_input"`
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "claude_code" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "claude_code",
		HookEvent:   p.HookEventName,
		SessionID:   p.SessionID,
		Workspace:   p.CWD,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch p.HookEventName {
	case "PermissionRequest":
		if p.ToolName == "AskUserQuestion" {
			base.Status = event.StatusInputRequired
			base.Title = notify.FormatTitle("claude_code", "input_required")
			base.Body = inputQuestionBody(p.ToolInput)
			return base, nil
		}
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("claude_code", "permission_required")
		base.Body = permissionBody(p.ToolName, p.ToolInput)
		return base, nil

	case "Notification":
		if isInputRequiredNotification(p.Message) {
			hint := extractInputHint(p.Message)
			base.Status = event.StatusInputRequired
			base.Title = notify.FormatTitle("claude_code", "input_required")
			base.Body = fmt.Sprintf("提示: %s", hint)
			return base, nil
		}
		return event.Event{}, fmt.Errorf("unsupported notification message: %s", p.Message)

	case "Stop":
		base.Status = event.StatusCompleted
		base.Title = notify.FormatTitle("claude_code", "run_completed")
		base.Body = notify.DefaultBody("run_completed")
		return base, nil

	case "PostToolUseFailure":
		errMsg := extractErrorMessage(p.ToolResponse)
		base.Status = event.StatusFailed
		base.Title = notify.FormatTitle("claude_code", "run_failed")
		base.Body = fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, errMsg)
		return base, nil

	default:
		return event.Event{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

// ── 辅助函数 ──────────────────────────────────────────────

func isInputRequiredNotification(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "waiting for your input") ||
		strings.Contains(msg, "waiting for input") ||
		strings.HasPrefix(msg, "needs input")
}

func extractInputHint(msg string) string {
	msg = strings.TrimSpace(msg)
	prefixes := []string{
		"claude is waiting for your input",
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

func inputQuestionBody(input map[string]any) string {
	question := firstQuestionText(input)
	if question == "" {
		return "提示: Claude Code 正在等待你的输入"
	}
	return fmt.Sprintf("提示: %s", truncateSummary(question, 1200))
}

func firstQuestionText(input map[string]any) string {
	questions, ok := input["questions"].([]any)
	if !ok || len(questions) == 0 {
		return ""
	}
	first, ok := questions[0].(map[string]any)
	if !ok {
		return ""
	}
	if question, ok := first["question"].(string); ok {
		return strings.TrimSpace(question)
	}
	return ""
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

func permissionBody(tool string, input map[string]any) string {
	if tool == "" {
		tool = "未知工具"
	}
	return fmt.Sprintf("工具: %s\n授权内容:\n%s", tool, summarizeToolInput(input))
}

func summarizeToolInput(input map[string]any) string {
	if len(input) == 0 {
		return "未提供工具参数"
	}
	for _, key := range []string{"command", "cmd", "description", "query", "pattern", "path", "file_path", "url", "prompt"} {
		value, ok := input[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return truncateSummary(text, 1200)
		}
	}
	raw, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "无法解析工具参数"
	}
	return truncateSummary(string(raw), 1200)
}

func truncateSummary(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "未提供工具参数"
	}
	if len(text) <= limit {
		return text
	}
	return text[:limit-20] + "\n...(已截断)"
}
