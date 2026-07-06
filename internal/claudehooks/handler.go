package claudehooks

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

	// 1. Remote approval owns PermissionRequest when available. In that case it
	// sends the approval card and writes Claude's hook decision, so a normal
	// notification would be a duplicate card.
	handled, err := agenthooks.MaybeHandleApproval(ctx, cfg, statePath, logPath, evt, stdout)
	if handled {
		return err
	}

	// 2. Plain notification path for events that remote approval did not own.
	if err := agenthooks.DispatchEvent(ctx, cfg, statePath, logPath, evt); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("dispatch error: %v", err))
	}

	return nil
}
