package version

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   command.FormatDescription(true, websiteUrl, false, description...),
		Long:    command.FormatDescription(false, websiteUrl, false, description...),
		Example: command.FormatExamples(examples...),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\n", version.Version())
			fmt.Printf("Time: %s\n", version.Time())
			fmt.Printf("Git commit ID: %s\n", version.Hash())
			return nil
		},
	}
}
