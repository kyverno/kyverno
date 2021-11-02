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

type mock struct {
	data map[string][]cosign.SignedPayload
}

func (m *mock) Verify(_ context.Context, signedImgRef name.Reference, accessor cosign.Accessor, _ *cosign.CheckOpts) ([]oci.Signature, bool, error) {
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
