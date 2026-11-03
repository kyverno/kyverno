package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
)

type options struct {
	imageRef   string
	digestOnly bool
}

func (o options) validate() error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	return nil
}

// execute fetches imageRef's manifest descriptor, checks it is a Kyverno bundle (D1: the shape
// check is free, the manifest is already fetched), and writes exactly one line to out: the
// digest-pinned reference by default, or the bare digest with --digest-only.
func (o options) execute(ctx context.Context, out io.Writer, keychain authn.Keychain) error {
	desc, ref, err := pull.FetchDescriptor(ctx, o.imageRef, keychain)
	if err != nil {
		return err
	}

	var manifest v1.Manifest
	if err := json.Unmarshal(desc.Manifest, &manifest); err != nil {
		return fmt.Errorf("resolve only pins Kyverno bundles: parsing manifest: %w; use \"crane digest\" to pin other artifacts", err)
	}
	if err := bundle.CheckManifestShape(&manifest); err != nil {
		return fmt.Errorf("resolve only pins Kyverno bundles: %w; use \"crane digest\" to pin other artifacts", err)
	}

	if o.digestOnly {
		fmt.Fprintln(out, desc.Digest.String())
		return nil
	}

	pinned := ref.Context().Digest(desc.Digest.String()).Name()
	fmt.Fprintln(out, pinned)
	return nil
}
