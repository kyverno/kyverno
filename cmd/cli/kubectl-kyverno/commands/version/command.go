package version

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", version.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "Time: %s\n", version.Time())
			fmt.Fprintf(cmd.OutOrStdout(), "Git commit ID: %s\n", version.Hash())
			return nil
		},
	}
}
