package bundle

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Metadata is the manifest-and-config-only view of a bundle ReadMetadata returns: a lighter read
// than Read, which also extracts and cross-checks the content layer.
type Metadata struct {
	Manifest *v1.Manifest
	// RawConfig is the config blob's bytes, verbatim, so a caller such as inspect's -o json can
	// echo the blob byte-for-byte rather than re-marshaling the typed Config.
	RawConfig []byte
	Config    Config
}

// ReadMetadata applies reader MUST 1, 6, and 7 over the manifest and config only: it never
// fetches or extracts the content layer and never cross-checks the index (reader MUST 2-4), so a
// caller that needs the archive verified uses Read or `kyverno oci validate -i`. It shares
// CheckManifestShape and parseConfig with Read, so the two entry points can never disagree on
// what a format-1.0 manifest and config blob look like.
func ReadMetadata(img v1.Image) (*Metadata, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	if err := CheckManifestShape(manifest); err != nil {
		return nil, err
	}

	cfg, rawConfig, err := parseConfig(img)
	if err != nil {
		return nil, err
	}

	// Reader MUST 6: reject a bundle whose major format version this reader doesn't support,
	// naming both versions (see Read's identical comment for why this lives here, against the
	// parsed config, rather than folded into CheckManifestShape).
	if err := checkFormatVersion(cfg.FormatVersion); err != nil {
		return nil, err
	}

	return &Metadata{
		Manifest:  manifest,
		RawConfig: rawConfig,
		Config:    cfg,
	}, nil
}
