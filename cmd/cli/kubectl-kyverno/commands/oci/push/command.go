package push

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var policyRef string
	var imageRef string
	cmd := &cobra.Command{
		Use:     "push",
		Short:   cobrautils.FormatDescription(true, websiteUrl, true, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, true, description...),
		Example: cobrautils.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			if imageRef == "" {
				return errors.New("image reference is required")
			}

			policies, _, err := policy.Load(nil, "", policyRef)
			if err != nil {
				return fmt.Errorf("unable to read policy file or directory %s: %w", policyRef, err)
			}

			openApiManager, err := openapi.NewManager(log.Log)
			if err != nil {
				return fmt.Errorf("creating openapi manager: %v", err)
			}
			for _, policy := range policies {
				if _, err := policyvalidation.Validate(policy, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName())); err != nil {
					return fmt.Errorf("validating policy %s: %v", policy.GetName(), err)
				}
			}

			img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
			img = mutate.ConfigMediaType(img, internal.PolicyConfigMediaType)
			ref, err := name.ParseReference(imageRef)
			if err != nil {
				return fmt.Errorf("parsing image reference: %v", err)
			}

			for _, policy := range policies {
				if policy.IsNamespaced() {
					fmt.Fprintf(os.Stderr, "Adding policy [%s]\n", policy.GetName())
				} else {
					fmt.Fprintf(os.Stderr, "Adding cluster policy [%s]\n", policy.GetName())
				}
				policyBytes, err := policyutils.ToYaml(policy)
				if err != nil {
					return fmt.Errorf("converting policy to yaml: %v", err)
				}
				policyLayer := static.NewLayer(policyBytes, internal.PolicyLayerMediaType)
				img, err = mutate.Append(img, mutate.Addendum{
					Layer:       policyLayer,
					Annotations: internal.Annotations(policy),
				})
				if err != nil {
					return fmt.Errorf("mutating image: %v", err)
				}
			}
			fmt.Fprintf(os.Stderr, "Uploading [%s]...\n", ref.Name())
			if err = remote.Write(ref, img, remote.WithContext(cmd.Context()), remote.WithAuthFromKeychain(keychain)); err != nil {
				return fmt.Errorf("writing image: %v", err)
			}
			fmt.Fprintf(os.Stderr, "Done.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&policyRef, "policy", "p", "", "path to policie(s)")
	cmd.Flags().StringVarP(&imageRef, "image", "i", "", "image reference to push to or pull from")
	return cmd
}
