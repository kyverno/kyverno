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
	"github.com/kyverno/kyverno/pkg/version"
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

	validateOpts := bundle.Options{
		Warn: func(msg string) { fmt.Fprintf(os.Stderr, "Warning: %s\n", msg) },
	}
	if err := bundle.ValidateWithOptions(b, validateOpts); err != nil {
		return err
	}

	d, err := bundle.LoadDescriptor(dir, b.Documents)
	if err != nil {
		return err
	}
	if d != nil {
		warnKyvernoVersionCompatibility(d.KyvernoVersion)
	}
	img, err := bundle.Write(b, d)
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

// warnKyvernoVersionCompatibility compares constraint against this CLI's own version and prints a
// warning or note to stderr; push never fails on the result (D5): the constraint describes the
// engine the bundle targets, not the tool packaging it.
func warnKyvernoVersionCompatibility(constraint string) {
	if constraint == "" {
		return
	}
	cliVersion := version.Version()
	c, err := bundle.CheckKyvernoVersion(constraint, cliVersion)
	if err != nil {
		// Unreachable for a bundle this CLI wrote: resolveDescriptor already rejected a malformed
		// constraint before Write. Kept defensive rather than panicking.
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		return
	}
	if text, _ := bundle.DescribeCompatibility(c, constraint, cliVersion); text != "" {
		fmt.Fprintln(os.Stderr, text)
	}
}
