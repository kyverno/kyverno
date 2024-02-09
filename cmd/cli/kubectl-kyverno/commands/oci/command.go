package oci

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/push"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		github.Keychain,
		registryclient.AWSKeychain,
		registryclient.GCPKeychain,
		registryclient.AzureKeychain,
	)
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
	return cmd
}
