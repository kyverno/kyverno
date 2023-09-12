package fix

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix/test"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "fix",
		Short:         command.FormatDescription(true, websiteUrl, true, description...),
		Long:          command.FormatDescription(false, websiteUrl, true, description...),
		Example:       command.FormatExamples(examples...),
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		test.Command(),
	)
	return cmd
}
