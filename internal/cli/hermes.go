package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newHermesCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hermes",
		Short: "Manage Hermes hook integration",
	}
	cmd.AddCommand(newHermesPrintHooksCmd(streams), newHermesInstallHooksCmd())
	return cmd
}

func newHermesPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print Hermes hook settings YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintHermesHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newHermesInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Hermes hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallHermesHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
