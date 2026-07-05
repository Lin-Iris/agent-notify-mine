package codexhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

var defaultAdapter = Adapter{}

func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("read stdin error: %v", err))
	}

	evt, err := defaultAdapter.Parse(data)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	handled, err := agenthooks.MaybeHandleApproval(ctx, cfg, statePath, logPath, evt, stdout)
	if handled {
		return err
	}

	return agenthooks.DispatchEvent(ctx, cfg, statePath, logPath, evt)
}
