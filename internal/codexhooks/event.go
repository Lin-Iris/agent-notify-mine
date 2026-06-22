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
		base.Body = fmt.Sprintf("工具: %s\n操作需要您的授权许可", fallbackToolName(p.ToolName))
		return base, nil

	case "Stop":
		// Stop 是生命周期信号，不是成功完成。
		// 状态机根据会话上下文推断最终状态。
		base.Status = event.StatusPending
		base.Title = notify.FormatTitle("codex", "running")
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
