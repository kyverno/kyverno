package create

import (
	metricsconfig "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/metrics-config"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/values"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Provides a command-line interface to help with the creation of various Kyverno resources.",
		Example: "",
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
