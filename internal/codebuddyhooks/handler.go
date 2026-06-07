package codebuddyhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// Handle 处理 CodeBuddy hook 的 stdin 输入，解析事件并分发通知。
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: read stdin error: %v", err))
	}

	msg, err := ParseMessage(data)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
