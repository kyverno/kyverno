package cosign

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
)

var client Cosign = &driver{}

type Cosign interface {
	VerifyImageSignatures(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error)
	VerifyImageAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) (checkedAttestations []oci.Signature, bundleVerified bool, err error)
}

type driver struct{}

func (d *driver) VerifyImageSignatures(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	return cosign.VerifyImageSignatures(ctx, signedImgRef, co)
}

func (d *driver) VerifyImageAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) (checkedAttestations []oci.Signature, bundleVerified bool, err error) {
	return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
}
