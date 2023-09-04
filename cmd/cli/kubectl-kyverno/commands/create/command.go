package create

import (
	metricsconfig "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/metrics-config"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/test"
	userinfo "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/user-info"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/values"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   cobrautils.FormatDescription(true, websiteUrl, false, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, false, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		metricsconfig.Command(),
		test.Command(),
		userinfo.Command(),
		values.Command(),
	)
	return cmd
}
