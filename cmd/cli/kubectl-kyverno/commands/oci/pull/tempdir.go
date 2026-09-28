package pull

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

// ToTempDir pulls the OCI artifact identified by imageRef into a new temporary
// directory. It returns the directory path, a cleanup function that removes the
// directory, and any error. The caller must invoke cleanup() when the directory
// is no longer needed.
func ToTempDir(ctx context.Context, imageRef string, keychain authn.Keychain) (string, func(), error) {
	if keychain == nil {
		keychain = NewKeychain()
	}
	dir, err := os.MkdirTemp("", "kyverno-oci-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }
	o := options{imageRef: imageRef}
	if err := o.execute(ctx, dir, keychain); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}
