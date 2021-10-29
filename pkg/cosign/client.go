package cosign

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
)

var client Cosign = &driver{}

type Cosign interface {
	Verify(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]cosign.SignedPayload, error)
}

type driver struct {
}

func (d *driver) Verify(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]cosign.SignedPayload, error) {
	return cosign.Verify(ctx, signedImgRef, co)
}
