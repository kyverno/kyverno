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
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/experimental"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kyverno",
		Long:  "To enable experimental commands, KYVERNO_EXPERIMENTAL should be configured with true or 1.",
		Short: "Kubernetes Native Policy Management",
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
