package notary

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/pkg/errors"
)

// ClusterPolicyAdapter adapts the new notary verifier to the old images.ImageVerifier interface
// This maintains backward compatibility for ClusterPolicy image verification
type ClusterPolicyAdapter struct {
	verifier *notary.Verifier
}

// init initializes the adapter with the new verifier
func (a *ClusterPolicyAdapter) init() images.ImageVerifier {
	a.verifier = notary.NewVerifier(logging.WithName("Notary"))
	return a
}

// VerifySignature implements images.ImageVerifier interface by adapting to the new verifier API
func (a *ClusterPolicyAdapter) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	// Create image data fetcher with nil secret lister (client provides auth)
	fetcher, err := imagedataloader.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create image data fetcher")
	}

	// Fetch image data - limitation: doesn't use client auth
	imageData, err := fetcher.FetchImageData(ctx, opts.ImageRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch image data for %s", opts.ImageRef)
	}

	// Combine certs and cert chain
	certsData := opts.Cert
	if opts.CertChain != "" {
		if certsData != "" {
			certsData = certsData + "\n"
		}
		certsData = certsData + opts.CertChain
	}

	// Call new verifier
	if err := a.verifier.VerifyImageSignature(ctx, imageData, certsData, ""); err != nil {
		return nil, err
	}

	// Return response
	return &images.Response{
		Digest:     imageData.Digest,
		Statements: nil,
	}, nil
}

// FetchAttestations implements images.ImageVerifier interface by adapting to the new verifier API
func (a *ClusterPolicyAdapter) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	// Create image data fetcher with nil secret lister (client provides auth)
	fetcher, err := imagedataloader.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create image data fetcher")
	}

	// Fetch image data - limitation: doesn't use client auth
	imageData, err := fetcher.FetchImageData(ctx, opts.ImageRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch image data for %s", opts.ImageRef)
	}

	// Combine certs and cert chain
	certsData := opts.Cert
	if opts.CertChain != "" {
		if certsData != "" {
			certsData = certsData + "\n"
		}
		certsData = certsData + opts.CertChain
	}

	// Determine referrer type from options
	referrerType := opts.Type
	if referrerType == "" && opts.PredicateType != "" {
		referrerType = opts.PredicateType
	}

	// Call new verifier
	if err := a.verifier.VerifyAttestationSignature(ctx, imageData, referrerType, certsData, ""); err != nil {
		return nil, err
	}

	// Extract statements from verified referrers
	statements := make([]map[string]interface{}, 0)

	// Try to get the payload using the attestation type
	if referrerType != "" {
		attestation := &policiesv1beta1.Attestation{
			Name: "adapter-attestation",
			Referrer: &policiesv1beta1.Referrer{
				Type: referrerType,
			},
		}
		payload, err := imageData.GetPayload(*attestation)
		if err == nil && payload != nil {
			if payloadMap, ok := payload.(map[string]interface{}); ok {
				statements = append(statements, payloadMap)
			}
		}
	}

	return &images.Response{
		Digest:     imageData.Digest,
		Statements: statements,
	}, nil
}
