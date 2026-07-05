package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCleanCmd(streams Streams) *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean agent-notify config, hooks, broker state, and logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanConfig(streams, purge); err != nil {
				return err
			}
			if purge {
				fmt.Fprintln(streams.Stdout, "agent-notify purge completed")
			} else {
				fmt.Fprintln(streams.Stdout, "agent-notify clean completed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "remove the whole ~/.agent-notify state directory after hooks are cleaned")
	return cmd
}
