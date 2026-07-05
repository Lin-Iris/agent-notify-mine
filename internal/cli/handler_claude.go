package cli

import (
	"context"

	"github.com/hellolib/agent-notify/internal/claudehooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleClaudeHookCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-claude-hook",
		Short:  "Internal Claude hook handler",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			return claudehooks.Handle(ctx, cfg, statePath, logPath, streams.Stdin, streams.Stdout)
		},
	}
}
