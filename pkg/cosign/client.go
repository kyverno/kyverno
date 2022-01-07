package cosign

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
)

var client Cosign = &driver{}

type Cosign interface {
	Verify(ctx context.Context, signedImgRef name.Reference, accessor cosign.Accessor, co *cosign.CheckOpts) ([]oci.Signature, bool, error)
}

type driver struct {
}

func (d *driver) Verify(ctx context.Context, signedImgRef name.Reference, accessor cosign.Accessor, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	return cosign.Verify(ctx, signedImgRef, accessor, co)
}
