package notary

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/pkg/errors"

	_ "github.com/kyverno/kyverno/pkg/logging" // Ensure logging is available for deprecation warnings
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
	logging.WithName("Notary").V(2).Info("DEPRECATION WARNING: pkg/notary is deprecated. This package delegates to pkg/imageverification/imageverifiers/notary for compatibility. Please migrate to ImageValidatingPolicy or update imports to use the new package directly.")

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

	certsData := opts.Cert
	if opts.CertChain != "" {
		if certsData != "" {
			certsData = certsData + "\n"
		}
		certsData = certsData + opts.CertChain
	}

	if err := a.verifier.VerifyImageSignature(ctx, imageData, certsData, ""); err != nil {
		return nil, err
	}

	return &images.Response{
		Digest:     imageData.Digest,
		Statements: nil,
	}, nil
}

// FetchAttestations implements images.ImageVerifier interface by adapting to the new verifier API
func (a *ClusterPolicyAdapter) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	logging.WithName("Notary").V(2).Info("DEPRECATION WARNING: pkg/notary is deprecated. This package delegates to pkg/imageverification/imageverifiers/notary for compatibility. Please migrate to ImageValidatingPolicy or update imports to use the new package directly.")

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

	certsData := opts.Cert
	if opts.CertChain != "" {
		if certsData != "" {
			certsData = certsData + "\n"
		}
		certsData = certsData + opts.CertChain
	}

	referrerType := opts.Type
	if referrerType == "" && opts.PredicateType != "" {
		referrerType = opts.PredicateType
	}

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
			// Wrap the payload in a statement structure expected by buildStatementMap
			// The structure should have:
			// - "type": the predicate type (used by buildStatementMap)
			// - "predicate": the predicate data (used by EvaluateConditions)
			statement := make(map[string]interface{})
			statement["type"] = referrerType

			if payloadMap, ok := payload.(map[string]interface{}); ok {
				statement["predicate"] = payloadMap
			} else {
				// If payload is not a map, wrap it as-is
				statement["predicate"] = payload
			}

			statements = append(statements, statement)
		}
	}

	return &images.Response{
		Digest:     imageData.Digest,
		Statements: statements,
	}, nil
}
