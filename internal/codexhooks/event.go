package codexhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Codex hooks 通过 stdin 投递的事件 JSON。
// 同时兼容 hook_event_name 与 event_name 两种字段名，
// 以及 last_assistant_message 与 last-assistant-message 两种字段名。
type payload struct {
	HookEventName        string         `json:"hook_event_name"`
	EventName            string         `json:"event_name"`
	SessionID            string         `json:"session_id"`
	CWD                  string         `json:"cwd"`
	Model                string         `json:"model"`
	PermissionMode       string         `json:"permission_mode"`
	TurnID               string         `json:"turn_id"`
	ToolName             string         `json:"tool_name"`
	ToolInput            map[string]any `json:"tool_input"`
	StopHookActive       bool           `json:"stop_hook_active"`
	LastAssistantMessage string         `json:"last_assistant_message"`
	// last-assistant-message（kebab-case）是 Codex 官方文档中的字段名
	LegacyLastAssistantMessage string `json:"last-assistant-message"`
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "codex" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	eventName := firstNonEmpty(p.HookEventName, p.EventName)
	canonicalEvent, err := normalizeHookEvent(eventName)
	if err != nil {
		return event.Event{}, err
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "codex",
		HookEvent:   canonicalEvent,
		SessionID:   p.SessionID,
		Workspace:   p.CWD,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch canonicalEvent {
	case "PermissionRequest":
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("codex", "permission_required")
		base.Body = permissionBody(p.ToolName, p.ToolInput)
		return base, nil

	case "Stop":
		base.Status = event.StatusCompleted
		base.Title = notify.FormatTitle("codex", "run_completed")
		body := notify.DefaultBody("run_completed")
		msg := firstNonEmpty(p.LastAssistantMessage, p.LegacyLastAssistantMessage)
		if hint := truncateMessage(strings.TrimSpace(msg), 200); hint != "" {
			body = hint
		}
		base.Body = body
		return base, nil

	default:
		return event.Event{}, fmt.Errorf("unsupported hook event: %s", eventName)
	}
}

// normalizeHookEvent keeps the state machine independent from Codex's wire
// spelling while remaining compatible with configs produced by older builds.
func normalizeHookEvent(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "permission_request", "permissionrequest":
		return "PermissionRequest", nil
	case "stop":
		return "Stop", nil
	default:
		return "", fmt.Errorf("unsupported hook event: %s", name)
	}
}

// ── 辅助函数 ──────────────────────────────────────────────

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func fallbackToolName(name string) string {
	if name == "" {
		return "未知工具"
	}
	return name
}

func truncateMessage(msg string, limit int) string {
	if msg == "" {
		return ""
	}
	if len(msg) <= limit {
		return msg
	}
	return msg[:limit-3] + "..."
}

func permissionBody(tool string, input map[string]any) string {
	return fmt.Sprintf("工具: %s\n授权内容:\n%s", fallbackToolName(tool), summarizeToolInput(input))
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
			return truncateMessage(text, 1200)
		}
	}
	raw, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "无法解析工具参数"
	}
	return truncateMessage(string(raw), 1200)
}
