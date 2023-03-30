package oci

import (
	"io"

	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/v1/google"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/spf13/cobra"
)

const (
	policyConfigMediaType = "application/vnd.cncf.kyverno.config.v1+json"
	policyLayerMediaType  = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
	annotationKind        = "io.kyverno.image.kind"
	annotationName        = "io.kyverno.image.name"
	annotationApiVersion  = "io.kyverno.image.apiVersion"
)

var (
	amazonKeychain = authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard)))
	azureKeychain  = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
	keychain       = authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		github.Keychain,
		amazonKeychain,
		azureKeychain,
	)
	imageRef string
)

func annotations(policy kyvernov1.PolicyInterface) map[string]string {
	kind := "ClusterPolicy"
	if policy.IsNamespaced() {
		kind = "Policy"
	}
	return map[string]string{
		annotationKind: kind,
		annotationName: policy.GetName(),
		// TODO: we need a way to get apiVersion
		annotationApiVersion: "kyverno.io/v1",
	}
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "oci",
		Long:    `This command is one of the supported experimental commands, and its behaviour might be changed any time.`,
		Short:   "Pulls/pushes images that include policie(s) from/to OCI registries.",
		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().StringVarP(&imageRef, "image", "i", "", "image reference to push to or pull from")
	cmd.AddCommand(ociPullCommand())
	cmd.AddCommand(ociPushCommand())
	return cmd
}
