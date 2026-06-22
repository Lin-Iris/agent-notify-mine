package codebuddyhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/state"
)

// preToolUsePayload 对应 CodeBuddy PreToolUse hook 传入的 JSON 结构
type preToolUsePayload struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
}

// 高风险工具列表 — 执行这些工具时发通知
var highRiskTools = map[string]string{
	"execute_command":       "执行命令",
	"write_to_file":         "写入文件",
	"delete_file":           "删除文件",
	"create_file":           "创建文件",
	"ask_followup_question": "向你提问",
	"edit_and_apply":        "编辑文件",
	"edit":                  "编辑文件",
}

// PreToolUseMatcher 是 PreToolUse hook 的 matcher 正则，匹配所有高风险工具
const PreToolUseMatcher = "execute_command|write_to_file|delete_file|create_file|ask_followup_question|edit_and_apply|edit"

// HandlePreToolUse 处理 PreToolUse hook 事件：解析 stdin -> 发通知 -> 输出 JSON 到 stdout
func HandlePreToolUse(ctx context.Context, cfg config.Config, statePath, logPath string, data []byte, stdout io.Writer) error {
	// 先输出 {} 到 stdout，确保 CodeBuddy 不阻塞（即使后续出错也不影响工具执行）
	defer func() {
		_, _ = fmt.Fprintln(stdout, "{}")
	}()

	var p preToolUsePayload
	if err := json.Unmarshal(data, &p); err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy pretooluse: parse error: %v", err))
	}

	// 只处理高风险工具
	action, ok := highRiskTools[p.ToolName]
	if !ok {
		return nil
	}

	// 提取工具参数中的关键信息
	detail := extractToolDetail(p.ToolName, p.ToolInput)

	// 构建统一 Event，使用 PreToolUse HookEvent 标识
	evt := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codebuddy",
		HookEvent:   "PreToolUse",
		Status:      event.StatusInputRequired,
		SessionID:   p.SessionID,
		Workspace:   p.CWD,
		Title:       "CodeBuddy " + action,
		Body:        fmt.Sprintf("工具: %s\n%s", p.ToolName, detail),
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	// 通过状态机分发通知
	if err := agenthooks.DispatchEvent(ctx, cfg, statePath, logPath, evt); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy pretooluse: dispatch error: %v", err))
	}

	return nil
}

// extractToolDetail 从工具输入中提取关键信息
func extractToolDetail(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "execute_command":
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			return truncate(cmd, 200)
		}
	case "write_to_file", "create_file":
		if path, ok := input["file_path"].(string); ok && path != "" {
			detail := "路径: " + path
			if content, ok := input["content"].(string); ok && content != "" {
				detail += "\n内容预览: " + truncate(content, 100)
			}
			return detail
		}
	case "delete_file":
		if path, ok := input["file_path"].(string); ok && path != "" {
			return "路径: " + path
		}
	case "edit_and_apply", "edit":
		if path, ok := input["file_path"].(string); ok && path != "" {
			return "路径: " + path
		}
		if path, ok := input["target_file"].(string); ok && path != "" {
			return "路径: " + path
		}
	case "ask_followup_question":
		var parts []string
		if question, ok := input["question"].(string); ok && question != "" {
			parts = append(parts, "问题: "+truncate(question, 200))
		}
		if questions, ok := input["questions"]; ok {
			if qs, ok := questions.([]any); ok && len(qs) > 0 {
				for i, q := range qs {
					if i >= 3 {
						parts = append(parts, fmt.Sprintf("...还有 %d 个问题", len(qs)-3))
						break
					}
					if qm, ok := q.(map[string]any); ok {
						if text, ok := qm["question"].(string); ok && text != "" {
							parts = append(parts, "问题: "+truncate(text, 200))
						}
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
