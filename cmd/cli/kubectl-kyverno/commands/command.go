package commands

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/docs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/version"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/experimental"
	"github.com/spf13/cobra"
)

func RootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kyverno",
		Short: cobrautils.FormatDescription(true, websiteUrl, description...),
		Long:  cobrautils.FormatDescription(false, websiteUrl, description...),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		apply.Command(),
		create.Command(),
		docs.Command(cmd),
		jp.Command(),
		test.Command(),
		version.Command(),
	)
	if experimental.IsExperimentalEnabled() {
		cmd.AddCommand(
			fix.Command(),
			oci.Command(),
		)
	}
	return cmd
}
