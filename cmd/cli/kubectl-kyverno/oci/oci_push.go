package oci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
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

			var p []string
			f, err := os.Stat(policyRef)
			if os.IsNotExist(err) {
				return fmt.Errorf("policy file or directory  %s does not exist", policyRef)
			}

			if f.IsDir() {
				err = filepath.Walk(policyRef, func(path string, info os.FileInfo, err error) error {
					if !info.IsDir() {
						p = append(p, path)
					}

					if m := info.Mode(); !(m.IsRegular() || m.IsDir()) {
						return nil
					}

					return nil
				})
			} else {
				p = append(p, policyRef)
			}

			if err != nil {
				return fmt.Errorf("unable to read policy file or directory %s: %w", policyRef, err)
			}

			fmt.Println("Policies will be pushing: ", p)

			img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
			img = mutate.ConfigMediaType(img, policyConfigMediaType)
			for _, policy := range p {
				policyBytes, err := os.ReadFile(filepath.Clean(policy))
				if err != nil {
					return fmt.Errorf("failed to read policy file %s: %v", policy, err)
				}

				var policyMap map[string]interface{}
				if err = yaml.Unmarshal(policyBytes, &policyMap); err != nil {
					return fmt.Errorf("failed to unmarshal policy file %s: %v", policy, err)
				}

				annotations := map[string]string{}
				for k, v := range policyMap["metadata"].(map[string]interface{})["annotations"].(map[string]interface{}) {
					annotations[k] = v.(string)
				}

				ref, err := name.ParseReference(imageRef)
				if err != nil {
					return fmt.Errorf("parsing image reference: %v", err)
				}

				do := []remote.Option{
					remote.WithContext(cmd.Context()),
					remote.WithAuthFromKeychain(keychain),
				}

				policyLayer := static.NewLayer(policyBytes, policyLayerMediaType)
				img, err = mutate.Append(img, mutate.Addendum{
					Layer:       policyLayer,
					Annotations: annotations,
				})

				if err != nil {
					return fmt.Errorf("mutating image: %v", err)
				}

				fmt.Fprintf(os.Stderr, "Uploading Kyverno policy file [%s] to [%s] with mediaType [%s].\n", policy, ref.Name(), policyLayerMediaType)
				if err = Write(ref, img, do...); err != nil {
					return fmt.Errorf("writing image: %v", err)
				}

				fmt.Fprintf(os.Stderr, "Kyverno policy file [%s] successfully uploaded to [%s]\n", policy, ref.Name())
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&policyRef, "policy", "p", "", "path to policie(s)")
	return cmd
}
