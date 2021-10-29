package cosign

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
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

func (m *mock) Verify(_ context.Context, signedImgRef name.Reference, _ *cosign.CheckOpts) ([]cosign.SignedPayload, error) {
	results, ok := m.data[signedImgRef.String()]
	if !ok {
		return nil, fmt.Errorf("failed to find mock data for %s", signedImgRef.String())
	}

	return results, nil
}
