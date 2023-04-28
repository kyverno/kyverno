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
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
			img = mutate.ConfigMediaType(img, policyConfigMediaType)
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
				policyLayer := static.NewLayer(policyBytes, policyLayerMediaType)
				img, err = mutate.Append(img, mutate.Addendum{
					Layer:       policyLayer,
					Annotations: annotations(policy),
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
	return cmd
}
