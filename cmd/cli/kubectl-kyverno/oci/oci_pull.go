package oci

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"github.com/spf13/cobra"
)

var dir string

func ociPullCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Long:  "This command is one of the supported experimental commands, and its behaviour might be changed any time",
		Short: "pulls policie(s) that are included in an OCI image from OCI registry and saves them to a local directory",
		Example: `# pull policy from an OCI image and save it to the specific directory
kyverno oci pull -i <imgref> -d policies`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if imageRef == "" {
				return errors.New("image reference is required")
			}

			dir = filepath.Clean(dir)
			if !filepath.IsAbs(dir) {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}

				dir, err = securejoin.SecureJoin(cwd, dir)
				if err != nil {
					return err
				}
			}

			fi, err := os.Lstat(dir)
			// Dir does not need to exist, as it can later be created.
			if err != nil && errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return fmt.Errorf("unable to create directory %s: %w", dir, err)
				}
			}

			if err == nil && !fi.IsDir() {
				return fmt.Errorf("dir '%s' must be a directory", dir)
			}

			ref, err := name.ParseReference(imageRef)
			if err != nil {
				return fmt.Errorf("parsing image reference: %v", err)
			}

			fmt.Fprintf(os.Stderr, "Downloading policies from an image [%s]...\n", ref.Name())
			rmt, err := remote.Get(ref, remote.WithContext(cmd.Context()), remote.WithAuthFromKeychain(keychain))
			if err != nil {
				return fmt.Errorf("getting image: %v", err)
			}

			img, err := rmt.Image()
			if err != nil {
				return fmt.Errorf("getting image: %v", err)
			}

			l, err := img.Layers()
			if err != nil {
				return fmt.Errorf("getting image layers: %v", err)
			}

			for _, layer := range l {
				lmt, err := layer.MediaType()
				if err != nil {
					return fmt.Errorf("getting layer media type: %v", err)
				}

				if lmt == policyLayerMediaType {
					blob, err := layer.Compressed()
					if err != nil {
						return fmt.Errorf("getting layer blob: %v", err)
					}
					defer blob.Close()

					layerBytes, err := io.ReadAll(blob)
					if err != nil {
						return fmt.Errorf("reading layer blob: %v", err)
					}
					policies, err := yamlutils.GetPolicy(layerBytes)
					if err != nil {
						return fmt.Errorf("unmarshaling layer blob: %v", err)
					}
					for _, policy := range policies {
						policyBytes, err := policyutils.ToYaml(policy)
						if err != nil {
							return fmt.Errorf("converting policy to yaml: %v", err)
						}
						pp := filepath.Join(dir, policy.GetName()+".yaml")
						fmt.Fprintf(os.Stderr, "Saving policy into disk [%s]...\n", pp)
						if err := os.WriteFile(pp, policyBytes, 0o600); err != nil {
							return fmt.Errorf("creating file: %v", err)
						}
					}
				}
			}
			fmt.Fprintf(os.Stderr, "Done.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "directory", "d", ".", "path to a directory")
	return cmd
}
