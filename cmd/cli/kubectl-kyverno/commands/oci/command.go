package oci

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/inspect"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/push"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/resolve"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/validate"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	keychain := pull.NewKeychain()
	cmd := &cobra.Command{
		Use:          "oci",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(pull.Command(keychain))
	cmd.AddCommand(push.Command(keychain))
	cmd.AddCommand(validate.Command(keychain))
	cmd.AddCommand(inspect.Command(keychain))
	cmd.AddCommand(resolve.Command(keychain))
	return cmd
}
