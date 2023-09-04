package fix

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix/test"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fix",
		Short:   cobrautils.FormatDescription(true, websiteUrl, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		test.Command(),
	)
	return cmd
}
