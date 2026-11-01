package push

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
)

type options struct {
	imageRef string
}

func (o options) validate(dir string) error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if dir == "" {
		return errors.New("policy is required")
	}
	return nil
}

func (o options) execute(ctx context.Context, dir string, keychain authn.Keychain) error {
	ref, err := name.ParseReference(o.imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %w", err)
	}

	b, err := bundle.Assemble(dir)
	if err != nil {
		return fmt.Errorf("assembling bundle from %s: %w", dir, err)
	}

	if err := bundle.Validate(b); err != nil {
		return err
	}

	// No descriptor support yet: #17663 parses kyverno-bundle.yaml into a *bundle.Descriptor
	// and passes it here. Until then every push uses the no-descriptor defaults.
	img, err := bundle.Write(b, nil)
	if err != nil {
		return fmt.Errorf("writing bundle image: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Uploading [%s]...\n", ref.Name())
	if err = remote.Write(ref, img, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain)); err != nil {
		return fmt.Errorf("writing image: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}
