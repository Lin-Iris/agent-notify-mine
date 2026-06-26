package hermeshooks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Hermes shell hook 通过 stdin 投递的 JSON。
type payload struct {
	HookEventName string       `json:"hook_event_name"`
	SessionID     string       `json:"session_id"`
	CWD           string       `json:"cwd"`
	ToolName      string       `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	Extra         extraPayload `json:"extra"`
}

type extraPayload struct {
	UserMessage       string `json:"user_message"`
	AssistantResponse string `json:"assistant_response"`
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "hermes" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "hermes",
		HookEvent:   p.HookEventName,
		SessionID:   p.SessionID,
		Workspace:   p.CWD,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch p.HookEventName {
	case "post_llm_call":
		base.Status = event.StatusPending
		base.Title = notify.FormatTitle("hermes", "run_completed")
		body := notify.DefaultBody("run_completed")
		if p.Extra.UserMessage != "" {
			body = fmt.Sprintf("任务: %s", truncateString(p.Extra.UserMessage, 200))
		}
		base.Body = body

	case "pre_approval_request":
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("hermes", "permission_required")
		body := p.ToolName
		if cmd, ok := p.ToolInput["command"]; ok {
			if cmdStr, ok := cmd.(string); ok {
				body = fmt.Sprintf("命令: %s", truncateString(cmdStr, 200))
			}
		}
		if body == "" {
			body = "需要您的授权许可"
		}
		base.Body = body

	default:
		return event.Event{}, fmt.Errorf("unsupported hermes hook event: %s", p.HookEventName)
	}

	return base, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
