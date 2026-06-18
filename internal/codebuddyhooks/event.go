package codebuddyhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hellolib/agent-notify/internal/notify"
)

// CodeBuddy hook 传入的 JSON 结构
type payload struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	Message       string         `json:"message"`
	ToolName      string         `json:"tool_name"`
	ToolResponse  map[string]any `json:"tool_response"`
	ToolInput     map[string]any `json:"tool_input"`
	// CodeBuddy Notification 事件的 matcher 字段
	Matcher string `json:"matcher"`
}

// ParseMessage 解析 CodeBuddy hook 传入的 JSON，转为统一的 Message 格式。
func ParseMessage(data []byte) (notify.Message, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.HookEventName {
	case "Stop":
		// CodeBuddy 的 Stop 在每次工具执行后都触发（bash/编辑等），
		// 不是真正的"任务完成"，跳过不发通知
		return notify.Message{}, fmt.Errorf("skip event: Stop（工具级别回调，不通知）")

	case "Notification":
		// CodeBuddy 的 Notification 通过 matcher 区分具体类型
		switch p.Matcher {
		case "permission_prompt":
			return notify.Message{
				Agent:     "codebuddy",
				Event:     "permission_required",
				SessionID: p.SessionID,
				Workspace: p.CWD,
				Title:     notify.FormatTitle("codebuddy", "permission_required"),
				Body:      fmt.Sprintf("工具: %s\n操作需要您的授权许可", p.ToolName),
			}, nil
		case "idle_prompt":
			return notify.Message{
				Agent:     "codebuddy",
				Event:     "input_required",
				SessionID: p.SessionID,
				Workspace: p.CWD,
				Title:     notify.FormatTitle("codebuddy", "input_required"),
				Body:      "CodeBuddy 空闲超过 60 秒，正在等待你的输入",
			}, nil
		default:
			// 其他 Notification 类型，仍然尝试根据 message 内容判断
			if isInputRequiredMessage(p.Message) {
				return notify.Message{
					Agent:     "codebuddy",
					Event:     "input_required",
					SessionID: p.SessionID,
					Workspace: p.CWD,
					Title:     notify.FormatTitle("codebuddy", "input_required"),
					Body:      fmt.Sprintf("提示: %s", extractHint(p.Message)),
				}, nil
			}
			return notify.Message{}, fmt.Errorf("unsupported notification matcher: %s", p.Matcher)
		}

	case "PostToolUseFailure":
		errMsg := extractErrorMessage(p.ToolResponse)
		return notify.Message{
			Agent:     "codebuddy",
			Event:     "run_failed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codebuddy", "run_failed"),
			Body:      fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, errMsg),
		}, nil

	case "SessionEnd":
		return notify.Message{
			Agent:     "codebuddy",
			Event:     "run_completed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codebuddy", "run_completed"),
			Body:      "CodeBuddy 会话已结束",
		}, nil

	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

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
