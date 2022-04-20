package cosign

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
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
		sigs = append(sigs, &sig{cosignPayload: sp})
	}

	return sigs, true, nil
}

type sig struct {
	oci.Signature
	cosignPayload cosign.SignedPayload
}

func (s *sig) Payload() ([]byte, error) {
	return s.cosignPayload.Payload, nil
}
