// Package event defines the canonical event protocol for agent-notify.
// Every raw hook event from any agent (Claude Code, Codex, CodeBuddy, etc.)
// is normalized into an Event before routing or state-machine processing.
package event

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SpecVersion identifies the event protocol version.
type SpecVersion string

const CurrentSpecVersion SpecVersion = "1.0"

// Status represents the inferred or best-guess status of an agent event.
// Adapters set a best-guess hint; the state machine may override.
type Status string

const (
	StatusPending       Status = "pending"            // 状态待定，需状态机推断（如 Stop）
	StatusPermissionReq Status = "permission_required" // 需要用户授权
	StatusInputRequired Status = "input_required"     // 等待用户输入
	StatusRunning       Status = "running"             // 运行中
	StatusCompleted     Status = "completed"           // 成功完成
	StatusFailed        Status = "failed"              // 执行失败
	StatusCancelled     Status = "cancelled"           // 被取消
)

// Event is the canonical protocol struct.
// Every hook JSON payload is normalized into this before routing or
// state-machine logic. RawPayload preserves the original JSON for
// future debugging, replay, or plugin hooks.
type Event struct {
	SpecVersion SpecVersion      `json:"spec_version"`
	EventID     string           `json:"event_id"`
	Agent       string           `json:"agent"`       // "claude_code", "codex", "codebuddy"
	HookEvent   string           `json:"hook_event"`  // 原始事件名，如 "Stop", "PermissionRequest"
	Status      Status           `json:"status"`       // 最佳猜测，状态机可覆盖
	SessionID   string           `json:"session_id"`
	Workspace   string           `json:"workspace"`
	Title       string           `json:"title"`        // 预格式化通知标题
	Body        string           `json:"body"`         // 预格式化通知正文
	RawPayload  json.RawMessage  `json:"raw_payload"`  // 原始 hook JSON
	ReceivedAt  time.Time        `json:"received_at"`
	// SkipFeishu 为 true 时，DispatchEvent 跳过飞书通知卡片。
	// 用于审批/输入流程已单独发送飞书卡片的场景，避免重复。
	SkipFeishu bool `json:"-"`
}

// Adapter normalizes raw hook JSON from a specific agent into an Event.
// Each xxxhooks package implements this interface.
type Adapter interface {
	// AgentName returns the agent identifier, e.g. "claude_code", "codex".
	AgentName() string

	// Parse reads raw hook JSON and returns a normalized Event.
	// The returned Event.Status is a best-guess hint; the state machine
	// may override it based on session context.
	Parse(raw []byte) (Event, error)
}

// NewEventID generates a unique event identifier.
func NewEventID() string {
	return uuid.New().String()
}

// StatusToEventName maps canonical Status values back to the notification-layer
// event name strings used by notify.Message and config event lists.
// This bridges the new event protocol with the existing notification pipeline.
func StatusToEventName(s Status) string {
	switch s {
	case StatusPermissionReq:
		return "permission_required"
	case StatusInputRequired:
		return "input_required"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "run_completed"
	case StatusFailed:
		return "run_failed"
	case StatusCancelled:
		return "run_cancelled"
	default:
		return "pending"
	}
}
