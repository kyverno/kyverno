package resolve

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
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

// execute fetches imageRef's manifest descriptor, checks it is a Kyverno bundle by reading its
// metadata (manifest shape plus config, reader MUST 1/6/7; no content layer is fetched), and
// writes exactly one line to out: the digest-pinned reference by default, or the bare digest with
// --digest-only.
func (o options) execute(ctx context.Context, out io.Writer, keychain authn.Keychain) error {
	desc, ref, err := pull.FetchDescriptor(ctx, o.imageRef, keychain)
	if err != nil {
		return err
	}

	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("resolve only pins Kyverno bundles: loading image from manifest: %w; use \"crane digest\" to pin other artifacts", err)
	}
	if _, err := bundle.ReadMetadata(img); err != nil {
		return fmt.Errorf("resolve only pins Kyverno bundles: %w; use \"crane digest\" to pin other artifacts", err)
	}

	if o.digestOnly {
		_, err := fmt.Fprintln(out, desc.Digest.String())
		return err
	}

	pinned := ref.Context().Digest(desc.Digest.String()).Name()
	_, err = fmt.Fprintln(out, pinned)
	return err
}
