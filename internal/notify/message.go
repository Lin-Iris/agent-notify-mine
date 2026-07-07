package notify

import "context"

type Message struct {
	Agent            string
	Event            string
	SessionID        string
	Workspace        string
	Profile          string
	Title            string
	Body             string
	RawPayload       string // 原始 hook JSON，用于调试和回放
	ApprovalID       string
	ApprovalToken    string
	InputRequestID   string
	InputToken       string
	InputPrompt      string
	InputOptions     []string
	InputMultiSelect bool
	InputAllowOther  bool
}

type Sender interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}
