package pull

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	cobrautils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/cobra"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"github.com/spf13/cobra"
)

func Command(keychain authn.Keychain) *cobra.Command {
	var dir string
	var imageRef string
	cmd := &cobra.Command{
		Use:     "pull",
		Short:   cobrautils.FormatDescription(true, websiteUrl, true, description...),
		Long:    cobrautils.FormatDescription(false, websiteUrl, true, description...),
		Example: cobrautils.FormatExamples(examples...),
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

				if lmt == internal.PolicyLayerMediaType {
					blob, err := layer.Compressed()
					if err != nil {
						return fmt.Errorf("getting layer blob: %v", err)
					}
					defer blob.Close()

					layerBytes, err := io.ReadAll(blob)
					if err != nil {
						return fmt.Errorf("reading layer blob: %v", err)
					}
					policies, _, err := yamlutils.GetPolicy(layerBytes)
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
	cmd.Flags().StringVarP(&imageRef, "image", "i", "", "image reference to push to or pull from")
	return cmd
}
