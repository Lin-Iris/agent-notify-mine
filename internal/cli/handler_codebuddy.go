package cli

import (
	"context"

	"github.com/hellolib/agent-notify/internal/codebuddyhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleCodeBuddyHookCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-codebuddy-hook",
		Short:  "Internal CodeBuddy hook handler",
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
			return codebuddyhooks.Handle(ctx, cfg, statePath, logPath, streams.Stdin)
		},
	}
}
