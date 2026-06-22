package codebuddyhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

var defaultAdapter = Adapter{}

func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: read stdin error: %v", err))
	}

	_ = state.AppendLog(logPath, fmt.Sprintf("codebuddy: raw stdin: %s", string(data)))

	evt, err := defaultAdapter.Parse(data)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("codebuddy: skip event: %v", err))
	}

	return agenthooks.DispatchEvent(ctx, cfg, statePath, logPath, evt)
}
