package version

import (
	"fmt"

	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   cobrautils.FormatDescription(true, websiteUrl, false, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, false, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\n", version.Version())
			fmt.Printf("Time: %s\n", version.Time())
			fmt.Printf("Git commit ID: %s\n", version.Hash())
			return nil
		},
	}
}
