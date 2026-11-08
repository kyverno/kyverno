package pull

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// FetchDescriptor resolves imageRef and fetches its manifest descriptor. desc.Digest is the
// digest the reference resolves to; desc.Manifest holds the manifest bytes. It rejects an image
// index: remote.Descriptor.Image() silently picks a platform child for an index, which would let
// a bundle wrapped in an index be accepted (violating reader MUST 1, the manifest at the
// reference must be an OCI image manifest) and would give a misleading config-media-type error
// for an ordinary multi-platform container image. This guard is the one shared primitive every
// caller (FetchImage, pull, validate -i, inspect, resolve) inherits.
func FetchDescriptor(ctx context.Context, imageRef string, keychain authn.Keychain) (*remote.Descriptor, name.Reference, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing image reference: %v", err)
	}
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, nil, fmt.Errorf("fetching remote image: %v", err)
	}
	if desc.MediaType != types.OCIManifestSchema1 {
		return nil, nil, fmt.Errorf("malformed bundle: expected an OCI 1.0 image manifest (mediaType %s), got %q", types.OCIManifestSchema1, desc.MediaType)
	}
	return desc, ref, nil
}

// FetchImage resolves imageRef and fetches the remote image, extracted from options.execute so
// kyverno oci validate's reference mode can reuse remote-fetch code without duplicating it.
func FetchImage(ctx context.Context, imageRef string, keychain authn.Keychain) (v1.Image, name.Reference, error) {
	desc, ref, err := FetchDescriptor(ctx, imageRef, keychain)
	if err != nil {
		return nil, nil, err
	}
	img, err := desc.Image()
	if err != nil {
		return nil, nil, fmt.Errorf("loading image from manifest: %v", err)
	}
	return img, ref, nil
}
