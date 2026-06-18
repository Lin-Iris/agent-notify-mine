package cli

import (
	"context"
	"io"

	"github.com/hellolib/agent-notify/internal/codebuddyhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleCodeBuddyPreToolUseCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-codebuddy-pretooluse",
		Short:  "Internal CodeBuddy PreToolUse hook handler",
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

			data, err := io.ReadAll(streams.Stdin)
			if err != nil {
				return err
			}

			return codebuddyhooks.HandlePreToolUse(ctx, cfg, statePath, logPath, data, streams.Stdout)
		},
	}
}
