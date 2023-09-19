package commands

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/docs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/version"
	"github.com/spf13/cobra"
)

func RootCommand(experimental bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kyverno",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
	if experimental {
		cmd.AddCommand(
			fix.Command(),
			oci.Command(),
		)
	}
	return cmd
}
