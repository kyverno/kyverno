package pull

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// FetchImage resolves imageRef and fetches the remote image, extracted from options.execute so
// kyverno oci validate's reference mode can reuse remote-fetch code without duplicating it.
func FetchImage(ctx context.Context, imageRef string, keychain authn.Keychain) (v1.Image, name.Reference, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing image reference: %v", err)
	}
	rmt, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, nil, fmt.Errorf("fetching remote image: %v", err)
	}
	img, err := rmt.Image()
	if err != nil {
		return nil, nil, fmt.Errorf("loading image from manifest: %v", err)
	}
	return img, ref, nil
}
