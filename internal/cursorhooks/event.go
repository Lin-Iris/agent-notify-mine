package cursorhooks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hellolib/agent-notify/internal/event"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Cursor 通过 stdin 投递的 hook JSON。
type payload struct {
	HookEventName  string   `json:"hook_event_name"`
	ConversationID string   `json:"conversation_id"`
	WorkspaceRoots []string `json:"workspace_roots"`
	Status         string   `json:"status"`         // stop 事件: "completed" | "error" | "aborted"
	ErrorMessage   string   `json:"error_message"`  // postToolUseFailure 事件
	ToolName       string   `json:"tool_name"`
	Command        string   `json:"command"`        // beforeShellExecution 事件
	CWD            string   `json:"cwd"`            // beforeShellExecution 事件
}

// ── Adapter ────────────────────────────────────────────────

type Adapter struct{}

func (a Adapter) AgentName() string { return "cursor" }

func (a Adapter) Parse(data []byte) (event.Event, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return event.Event{}, err
	}

	workspace := ""
	if len(p.WorkspaceRoots) > 0 {
		workspace = p.WorkspaceRoots[0]
	}

	base := event.Event{
		SpecVersion: event.CurrentSpecVersion,
		EventID:     event.NewEventID(),
		Agent:       "cursor",
		HookEvent:   p.HookEventName,
		SessionID:   p.ConversationID,
		Workspace:   workspace,
		RawPayload:  json.RawMessage(data),
		ReceivedAt:  time.Now(),
	}

	switch p.HookEventName {
	case "stop":
		switch p.Status {
		case "completed":
			base.Status = event.StatusPending
			base.Title = notify.FormatTitle("cursor", "run_completed")
			base.Body = notify.DefaultBody("run_completed")
		case "error", "aborted":
			base.Status = event.StatusFailed
			base.Title = notify.FormatTitle("cursor", "run_failed")
			base.Body = fmt.Sprintf("任务状态: %s", p.Status)
		default:
			base.Status = event.StatusPending
			base.Title = notify.FormatTitle("cursor", "run_completed")
			base.Body = notify.DefaultBody("run_completed")
		}

	case "beforeShellExecution":
		base.Status = event.StatusPermissionReq
		base.Title = notify.FormatTitle("cursor", "permission_required")
		body := fmt.Sprintf("命令: %s", truncateString(p.Command, 200))
		if p.CWD != "" {
			body += fmt.Sprintf("\n目录: %s", p.CWD)
		}
		base.Body = body

	case "postToolUseFailure":
		errMsg := p.ErrorMessage
		if errMsg == "" {
			errMsg = "工具执行失败"
		}
		base.Status = event.StatusFailed
		base.Title = notify.FormatTitle("cursor", "run_failed")
		base.Body = fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, truncateString(errMsg, 200))

	default:
		return event.Event{}, fmt.Errorf("unsupported cursor hook event: %s", p.HookEventName)
	}

	return base, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
