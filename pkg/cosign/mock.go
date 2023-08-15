package cosign

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
)

func SetMock(image string, data [][]byte) error {
	imgRef, err := name.ParseReference(image)
	if err != nil {
		return err
	}

	payloads := make([]cosign.SignedPayload, len(data))
	for i, p := range data {
		payloads[i] = cosign.SignedPayload{
			Payload: p,
		}
	}

	client = &mock{data: map[string][]cosign.SignedPayload{
		imgRef.String(): payloads,
	}}

	return nil
}

func ClearMock() {
	client = &driver{}
}

type mock struct {
	data map[string][]cosign.SignedPayload
}

func (m *mock) VerifyImageSignatures(_ context.Context, signedImgRef name.Reference, _ *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	return m.getSignatures(signedImgRef)
}

func (m *mock) VerifyImageAttestations(ctx context.Context, signedImgRef name.Reference, co *cosign.CheckOpts) (checkedAttestations []oci.Signature, bundleVerified bool, err error) {
	return m.getSignatures(signedImgRef)
}

func (m *mock) getSignatures(signedImgRef name.Reference) ([]oci.Signature, bool, error) {
	results, ok := m.data[signedImgRef.String()]
	if !ok {
		return nil, false, fmt.Errorf("failed to find mock data for %s", signedImgRef.String())
	}

	sigs := make([]oci.Signature, 0, len(results))
	for _, sp := range results {
		ociSig, err := getSignature(sp)
		if err != nil {
			return nil, false, err
		}
		sigs = append(sigs, ociSig)
	}

	return sigs, true, nil
}

func getSignature(sp cosign.SignedPayload) (oci.Signature, error) {
	chain := make([]byte, 0)
	for _, cert := range sp.Chain {
		chain = append(chain, cert.Raw...)
	}
	staticOpts := []static.Option{}
	if sp.Cert != nil {
		staticOpts = append(staticOpts, static.WithCertChain(sp.Cert.Raw, chain))
	}
	if sp.Bundle != nil {
		staticOpts = append(staticOpts, static.WithBundle(sp.Bundle))
	}
	ociSig, err := static.NewSignature(sp.Payload, sp.Base64Signature, staticOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get signature %v", err)
	}
	return ociSig, nil
}
