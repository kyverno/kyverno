package oci

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
)

var policyRef string

func ociPushCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Long:  "This command is one of the supported experimental commands in Kyverno CLI, and its behaviour might be changed any time.",
		Short: "push policie(s) that are included in an OCI image to OCI registry",
		Example: `# push policy to an OCI image from a given policy file
kyverno oci push -p policy.yaml -i <imgref>

# push multiple policies to an OCI image from a given directory that includes policies
kyverno oci push -p policies. -i <imgref>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if imageRef == "" {
				return errors.New("image reference is required")
			}

			policies, errs := common.GetPolicies([]string{policyRef})
			if len(errs) != 0 {
				return fmt.Errorf("unable to read policy file or directory %s: %w", policyRef, multierr.Combine(errs...))
			}

			// TODO: validate policy

			img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
			img = mutate.ConfigMediaType(img, policyConfigMediaType)
			ref, err := name.ParseReference(imageRef)
			if err != nil {
				return fmt.Errorf("parsing image reference: %v", err)
			}

			for _, policy := range policies {
				if policy.IsNamespaced() {
					fmt.Println("Adding policy:", policy.GetName(), "...")
				} else {
					fmt.Println("Adding cluster policy", policy.GetName(), "...")
				}
				policyBytes, err := yaml.Marshal(policy)
				if err != nil {
					return fmt.Errorf("converting policy to yaml: %v", err)
				}
				policyLayer := static.NewLayer(policyBytes, policyLayerMediaType)
				img, err = mutate.Append(img, mutate.Addendum{
					Layer:       policyLayer,
					Annotations: policy.GetAnnotations(),
				})
				if err != nil {
					return fmt.Errorf("mutating image: %v", err)
				}
			}
			fmt.Fprintf(os.Stderr, "Uploading [%s]...\n", ref.Name())
			if err = Write(ref, img, remote.WithContext(cmd.Context()), remote.WithAuthFromKeychain(keychain)); err != nil {
				return fmt.Errorf("writing image: %v", err)
			}
			fmt.Fprintf(os.Stderr, "Done.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&policyRef, "policy", "p", "", "path to policie(s)")
	return cmd
}
