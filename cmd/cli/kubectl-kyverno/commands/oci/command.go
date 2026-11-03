package oci

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/inspect"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/push"
<<<<<<< HEAD
=======
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/resolve"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/validate"
>>>>>>> 9a39e3a58 (feat(cli): add kyverno oci inspect and oci resolve (#18420))
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	keychain := pull.NewKeychain()
	cmd := &cobra.Command{
		Use:          "oci",
		Short:        command.FormatDescription(true, websiteUrl, true, description...),
		Long:         command.FormatDescription(false, websiteUrl, true, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(pull.Command(keychain))
	cmd.AddCommand(push.Command(keychain))
<<<<<<< HEAD
=======
	cmd.AddCommand(validate.Command(keychain))
	cmd.AddCommand(inspect.Command(keychain))
	cmd.AddCommand(resolve.Command(keychain))
>>>>>>> 9a39e3a58 (feat(cli): add kyverno oci inspect and oci resolve (#18420))
	return cmd
}
