package pull

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
)

type options struct {
	imageRef string
}

func (o options) validate(dir string) error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if dir == "" {
		return errors.New("dir is required")
	}
	return nil
}

func (o options) execute(ctx context.Context, dir string, keychain authn.Keychain) error {
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
	ref, err := name.ParseReference(o.imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Downloading policies from an image [%s]...\n", ref.Name())
	rmt, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
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
			if err := extractAndSavePolicies(layer, dir); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}

// extractAndSavePolicies reads the policies out of a single layer's blob and writes them to
// disk. It is factored out of execute so the layer's ReadCloser is closed at the end of each
// iteration rather than accumulating open readers until execute returns.
func extractAndSavePolicies(layer v1.Layer, dir string) error {
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("getting layer blob: %v", err)
	}
	defer blob.Close()

	layerBytes, err := io.ReadAll(blob)
	if err != nil {
		return fmt.Errorf("reading layer blob: %v", err)
	}
	policies, _, _, _, _, _, _, err := yamlutils.GetPolicy(layerBytes)
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
	return nil
}
