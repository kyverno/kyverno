package verifiers

import (
	"context"
)

type ImageVerifier interface {
	// VerifySignature verifies that the image has the expected signatures
	VerifySignature(ctx context.Context, opts Options) (*Response, error)
	// FetchAttestations retrieves signed attestations and decodes them into in-toto statements
	// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
	FetchAttestations(ctx context.Context, opts Options) (*Response, error)
}
