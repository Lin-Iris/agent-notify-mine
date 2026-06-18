package cli

import (
	"context"
	"time"

	"github.com/hellolib/agent-notify/internal/codebuddyhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleCodeBuddyFireStopCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-codebuddy-fire-stop",
		Short:  "Internal: fire CodeBuddy debounced Stop notification",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 先等待防抖窗口，确保没有新的 Stop 事件进来
			time.Sleep(codebuddyhooks.DebounceSeconds * time.Second)

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			statePath, err := config.StatePath()
			if err != nil {
				return err
			}
			logPath, err := config.LogPath()
			if err != nil {
				return err
			}

			expectedTimestamp := ""
			if len(args) > 0 {
				expectedTimestamp = args[0]
			}

			return codebuddyhooks.CheckAndFire(cmd.Context(), cfg, statePath, logPath, expectedTimestamp)
		},
	}
}
