package fix

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix/test"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fix",
		Short:   "Provides a command-line interface to fix inconsistencies and deprecated usage of Kyverno resources.",
		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		test.Command(),
	)
	return cmd
}
