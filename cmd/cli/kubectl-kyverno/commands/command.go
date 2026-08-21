package commands

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/completion"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/docs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/jp"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/migrate"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/version"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/spf13/cobra"
)

func RootCommand(experimental bool) *cobra.Command {
	var warningsAsErrors bool
	cmd := &cobra.Command{
		Use:          "kyverno",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		SilenceUsage: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			policy.WarningsAsErrors = warningsAsErrors
			exception.WarningsAsErrors = warningsAsErrors
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().BoolVar(&warningsAsErrors, "warnings-as-errors", false, "fail the command if deprecated API versions or fields are detected")
	cmd.AddCommand(
		apply.Command(),
		completion.Command(),
		create.Command(),
		docs.Command(cmd),
		jp.Command(),
		migrate.Command(),
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
