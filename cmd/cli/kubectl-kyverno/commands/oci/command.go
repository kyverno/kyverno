package oci

import (
	"io"

	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/push"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	amazonKeychain := authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard)))
	azureKeychain := authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
		amazonKeychain,
		azureKeychain,
	)
	cmd := &cobra.Command{
		Use:     "oci",
		Short:   cobrautils.FormatDescription(true, websiteUrl, false, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, false, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(pull.Command(keychain))
	cmd.AddCommand(push.Command(keychain))
	return cmd
}
