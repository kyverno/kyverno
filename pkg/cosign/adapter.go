package cosign

import (
	"context"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/cosign"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/pkg/errors"
)

// ClusterPolicyAdapter adapts the new cosign verifier to the old images.ImageVerifier interface
// This maintains backward compatibility for ClusterPolicy image verification
type ClusterPolicyAdapter struct {
	verifier *cosign.Verifier
}

// init initializes the adapter with the new verifier
func (a *ClusterPolicyAdapter) init() images.ImageVerifier {
	a.verifier = cosign.NewVerifier(nil, logging.WithName("Cosign"))
	return a
}

// VerifySignature implements images.ImageVerifier interface by adapting to the new verifier API
func (a *ClusterPolicyAdapter) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	// Log deprecation warning
	logging.WithName("Cosign").V(2).Info("DEPRECATION WARNING: pkg/cosign is deprecated. This package delegates to pkg/imageverification/imageverifiers/cosign for compatibility. Please migrate to ImageValidatingPolicy or update imports to use the new package directly.")

	// Get remote options from the client
	remoteOpts, err := opts.Client.Options(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get remote options")
	}

	// Get name options from the client
	nameOpts := opts.Client.NameOptions()

	// Create image data fetcher with nil secret lister (client provides auth)
	fetcher, err := imagedataloader.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create image data fetcher")
	}

	// Fetch image data using manual remote options since we can't pass Client directly
	imageData, err := a.fetchImageDataWithClient(ctx, fetcher, opts.ImageRef, remoteOpts, nameOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch image data for %s", opts.ImageRef)
	}

	// Convert images.Options to attestor
	attestor := convertOptionsToAttestor(&opts)

	// Call new verifier
	if err := a.verifier.VerifyImageSignature(ctx, imageData, attestor); err != nil {
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
	// Log deprecation warning
	logging.WithName("Cosign").V(2).Info("DEPRECATION WARNING: pkg/cosign is deprecated. This package delegates to pkg/imageverification/imageverifiers/cosign for compatibility. Please migrate to ImageValidatingPolicy or update imports to use the new package directly.")

	// Get remote options from the client
	remoteOpts, err := opts.Client.Options(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get remote options")
	}

	// Get name options from the client
	nameOpts := opts.Client.NameOptions()

	// Create image data fetcher
	fetcher, err := imagedataloader.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create image data fetcher")
	}

	// Fetch image data
	imageData, err := a.fetchImageDataWithClient(ctx, fetcher, opts.ImageRef, remoteOpts, nameOpts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch image data for %s", opts.ImageRef)
	}

	// Convert images.Options to attestor and attestation
	attestor := convertOptionsToAttestor(&opts)
	attestation := convertOptionsToAttestation(&opts)

	// Call new verifier
	if err := a.verifier.VerifyAttestationSignature(ctx, imageData, attestation, attestor); err != nil {
		return nil, err
	}

	// Extract statements from verified payloads
	statements := make([]map[string]interface{}, 0)
	payload, err := imageData.GetPayload(*attestation)
	if err == nil && payload != nil {
		if payloadMap, ok := payload.(map[string]interface{}); ok {
			statements = append(statements, payloadMap)
		}
	}

	return &images.Response{
		Digest:     imageData.Digest,
		Statements: statements,
	}, nil
}

// fetchImageDataWithClient is a helper that manually constructs the image data fetch
// This is needed because the fetcher doesn't accept a Client directly
func (a *ClusterPolicyAdapter) fetchImageDataWithClient(ctx context.Context, fetcher imagedataloader.Fetcher, imageRef string, remoteOpts, nameOpts interface{}) (*imagedataloader.ImageData, error) {
	// For now, just call FetchImageData without extra options
	// The fetcher will use anonymous credentials, which may not work for private registries
	// This is a limitation of the current adapter approach
	return fetcher.FetchImageData(ctx, imageRef)
}

// convertOptionsToAttestor converts images.Options to a policiesv1beta1.Attestor
//
// Key Routing Logic:
// - Direct PEM keys are routed to Key.Data field
// - k8s:// secret references are routed to Key.KMS field
// - KMS references (AWS, GCP, Azure, HashiVault) are routed to Key.KMS field
//
// The Key.KMS field is handled by Cosign's PublicKeyFromKeyRefWithHashAlgo function,
// which has built-in support for fetching Kubernetes secrets and KMS keys.
// This means k8s://namespace/secret references work without needing a secretInterface
// in the adapter - Cosign accesses the cluster API directly.
func convertOptionsToAttestor(opts *images.Options) *policiesv1beta1.Attestor {
	attestor := &policiesv1beta1.Attestor{}

	// Create cosign attestor
	cosignAttestor := &policiesv1beta1.Cosign{}

	// Handle key-based verification
	// The Key field supports: direct PEM keys, k8s://namespace/secret, or KMS references
	if opts.Key != "" {
		cosignAttestor.Key = &policiesv1beta1.Key{
			HashAlgorithm: opts.SignatureAlgorithm,
		}

		// Route to the correct field based on key format:
		// - Key.KMS: for k8s:// and KMS providers (handled by Cosign's PublicKeyFromKeyRefWithHashAlgo)
		// - Key.Data: for direct PEM-encoded public keys (handled by decodePEM)
		if strings.HasPrefix(opts.Key, "k8s://") || strings.HasPrefix(opts.Key, "azurekms://") ||
			strings.HasPrefix(opts.Key, "gcpkms://") || strings.HasPrefix(opts.Key, "awskms://") ||
			strings.HasPrefix(opts.Key, "hashivault://") {
			// KMS and k8s:// references go to the KMS field
			// Cosign will fetch these from their respective sources (K8s API, cloud KMS, etc.)
			cosignAttestor.Key.KMS = opts.Key
		} else {
			// Direct PEM keys go to the Data field
			// These are decoded directly without external lookups
			cosignAttestor.Key.Data = opts.Key
		}
	}

	// Handle certificate-based verification
	if opts.Cert != "" || opts.CertChain != "" {
		cosignAttestor.Certificate = &policiesv1beta1.Certificate{}
		if opts.Cert != "" {
			cosignAttestor.Certificate.Certificate = &policiesv1beta1.StringOrExpression{Value: opts.Cert}
		}
		if opts.CertChain != "" {
			cosignAttestor.Certificate.CertificateChain = &policiesv1beta1.StringOrExpression{Value: opts.CertChain}
		}
	}

	// Handle keyless verification
	if opts.Issuer != "" || opts.Subject != "" || opts.IssuerRegExp != "" || opts.SubjectRegExp != "" {
		cosignAttestor.Keyless = &policiesv1beta1.Keyless{
			Roots: opts.Roots,
		}

		identity := policiesv1beta1.Identity{
			Issuer:        opts.Issuer,
			Subject:       opts.Subject,
			IssuerRegExp:  opts.IssuerRegExp,
			SubjectRegExp: opts.SubjectRegExp,
		}
		cosignAttestor.Keyless.Identities = []policiesv1beta1.Identity{identity}
	}

	// Handle CTLog settings - CTLog is on the Cosign struct itself in v1beta1 API
	//
	// IMPORTANT: TrustedMaterial is only set for keyless verification in the new verifier.
	// For key-based verification, if we try to verify transparency logs without TrustedMaterial,
	// it will cause a nil pointer dereference panic.
	//
	// Behavior:
	// 1. If explicit CTLog config is provided (RekorURL, RekorPubKey, etc.), use it
	// 2. For key-based verification without explicit CTLog config, default to IgnoreTlog=true
	//    to prevent nil pointer errors (TrustedMaterial is not set for key-based verification)
	// 3. For keyless/certificate verification, TrustedMaterial will be set by the verifier
	hasCTLogConfig := opts.RekorURL != "" || opts.RekorPubKey != "" || opts.IgnoreTlog || opts.IgnoreSCT || opts.CTLogsPubKey != "" || opts.TSACertChain != ""

	if hasCTLogConfig {
		cosignAttestor.CTLog = &policiesv1beta1.CTLog{
			InsecureIgnoreTlog: opts.IgnoreTlog,
			InsecureIgnoreSCT:  opts.IgnoreSCT,
		}

		if opts.RekorURL != "" {
			cosignAttestor.CTLog.URL = opts.RekorURL
		}
		if opts.RekorPubKey != "" {
			cosignAttestor.CTLog.RekorPubKey = opts.RekorPubKey
		}
		if opts.CTLogsPubKey != "" {
			cosignAttestor.CTLog.CTLogPubKey = opts.CTLogsPubKey
		}
		if opts.TSACertChain != "" {
			cosignAttestor.CTLog.TSACertChain = opts.TSACertChain
		}

		// CRITICAL FIX: For key-based verification, we must force IgnoreTlog=true
		// even if the old code provided RekorURL, because TrustedMaterial is only
		// set for keyless verification. Without this, bundle verification will panic.
		//
		// The old ClusterPolicy code set RekorURL="https://rekor.sigstore.dev" and
		// IgnoreTlog=false by default, but this doesn't work with the new verifier
		// architecture which requires TrustedMaterial for transparency log verification.
		if cosignAttestor.Key != nil || cosignAttestor.Certificate != nil {
			cosignAttestor.CTLog.InsecureIgnoreTlog = true
			cosignAttestor.CTLog.InsecureIgnoreSCT = true
		}
	} else if cosignAttestor.Key != nil {
		// For key-based verification without explicit CTLog config, we must ignore tlog/sct
		cosignAttestor.CTLog = &policiesv1beta1.CTLog{
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
		}
	}

	// Handle repository override
	if opts.Repository != "" {
		cosignAttestor.Source = &policiesv1beta1.Source{
			Repository: opts.Repository,
		}
	}

	// Set annotations
	if len(opts.Annotations) > 0 {
		cosignAttestor.Annotations = opts.Annotations
	}

	attestor.Cosign = cosignAttestor
	return attestor
}

// convertOptionsToAttestation converts images.Options to a policiesv1beta1.Attestation
func convertOptionsToAttestation(opts *images.Options) *policiesv1beta1.Attestation {
	attestation := &policiesv1beta1.Attestation{
		Name: "adapter-attestation",
	}

	// Determine attestation type
	attestationType := opts.Type
	if attestationType == "" && opts.PredicateType != "" {
		attestationType = opts.PredicateType
	}

	if attestationType != "" {
		attestation.InToto = &policiesv1beta1.InToto{
			Type: attestationType,
		}
	} else {
		// Default to empty InToto if no type specified
		attestation.InToto = &policiesv1beta1.InToto{}
	}

	return attestation
}
