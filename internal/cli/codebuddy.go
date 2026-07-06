package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newCodeBuddyCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codebuddy",
		Short: "Manage CodeBuddy hook integration",
	}
	cmd.AddCommand(newCodeBuddyPrintHooksCmd(streams), newCodeBuddyInstallHooksCmd())
	return cmd
}

func newCodeBuddyPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print CodeBuddy hook settings JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintCodeBuddyHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newCodeBuddyInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install CodeBuddy hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallCodeBuddyHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
