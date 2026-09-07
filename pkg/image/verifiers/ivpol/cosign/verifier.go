package cosign

import (
	"context"
	"crypto/x509"
	stderrors "errors"
	"fmt"
	"net"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/policy"
	"github.com/sigstore/sigstore-go/pkg/root"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type Verifier struct {
	secretLister corev1listers.SecretLister
	log          logr.Logger
}

func NewVerifier(secretLister corev1listers.SecretLister, logger logr.Logger) *Verifier {
	return &Verifier{
		log:          logging.WithName("Cosign"),
		secretLister: secretLister,
	}
}

// buildCheckOptsWithBundleDetection builds CheckOpts and auto-detects cosign v3 bundle format
func (v *Verifier) buildCheckOptsWithBundleDetection(ctx context.Context, attestor *policiesv1beta1.Cosign, image *imagedataloader.ImageData) (*cosign.CheckOpts, error) {
	cOpts, err := checkOptions(ctx, attestor, image.RemoteOpts(), image.NameOpts(), v.secretLister)
	if err != nil {
		return nil, err
	}

	// Detect the cosign v3 bundle format. A benign "no bundle" result falls back
	// to the legacy format; any other error is an infra failure and must surface.
	newBundles, _, err := cosign.GetBundles(ctx, image.NameRef(), cOpts.RegistryClientOpts)
	if err != nil && !isNoBundle(err) {
		return nil, errors.Wrapf(err, "failed to detect cosign bundle format")
	}
	bundleDetected := len(newBundles) > 0
	cOpts.NewBundleFormat = bundleDetected
	if bundleDetected && shouldUseSignedTimestamps(cOpts.IgnoreTlog, cOpts.UseSignedTimestamps, cOpts.TrustedMaterial) {
		cOpts.UseSignedTimestamps = true
	}

	return cOpts, nil
}

// shouldUseSignedTimestamps reports whether a detected Sigstore bundle (format
// v0.3, used e.g. by GitHub Actions) should be verified using its embedded
// RFC 3161 signed timestamp instead of the current time.
//
// Sigstore bundles carry an RFC 3161 timestamp proving the signing time. When
// the transparency log is ignored, cosign falls back to verifying the
// short-lived Fulcio leaf certificate against the current time
// (verificationOptions() -> WithCurrentTime()), which rejects any certificate
// outside its ~10 minute validity window and makes historical attestations
// unverifiable. Using the bundle's signed timestamp instead (verified against
// the TSAs in the trusted root) allows verifying the certificate was valid at
// signing time.
//
// trustedMaterial must be populated (it is nil when checkOptions took the
// skipSigstoreInfra fast path for key/cert attestors), since signed-timestamp
// verification needs the trusted root's timestamp authorities; without it
// cosign/sigstore-go panics on a nil TrustedMaterial rather than returning an
// error.
func shouldUseSignedTimestamps(ignoreTlog, useSignedTimestamps bool, trustedMaterial root.TrustedMaterial) bool {
	return ignoreTlog && !useSignedTimestamps && trustedMaterial != nil
}

// isNoBundle reports whether a GetBundles error is a benign "no v3 bundle"
// result (empty referrers index or missing tag) rather than an infra failure.
func isNoBundle(err error) bool {
	var noBundles *cosign.ErrNoMatchingAttestations
	var tagNotFound *cosign.ErrImageTagNotFound
	return stderrors.As(err, &noBundles) || stderrors.As(err, &tagNotFound)
}

// classifyVerifyError tags a verify-call failure as a SetupError only when it is
// a recognizable infrastructure failure; anything else (including unrecognized
// errors) is treated as a genuine verification failure and fails closed.
func classifyVerifyError(err error) error {
	wrapped := errors.Wrapf(err, "failed to verify cosign signatures")
	if isInfraError(err) {
		return Setup(wrapped)
	}
	return wrapped
}

// isInfraError reports whether err is a registry/network/TLS/timeout failure.
// It matches only on the transport type and the standard library (not cosign's
// verification-error types) so it stays stable across cosign upgrades.
func isInfraError(err error) bool {
	if err == nil {
		return false
	}
	var te *transport.Error // registry HTTP errors (401/403/5xx/429...)
	var netErr net.Error
	var urlErr *url.Error
	var unknownAuthority x509.UnknownAuthorityError
	var certInvalid x509.CertificateInvalidError
	var hostErr x509.HostnameError
	return stderrors.As(err, &te) ||
		stderrors.As(err, &netErr) ||
		stderrors.As(err, &urlErr) ||
		stderrors.As(err, &unknownAuthority) ||
		stderrors.As(err, &certInvalid) ||
		stderrors.As(err, &hostErr) ||
		stderrors.Is(err, context.DeadlineExceeded) ||
		stderrors.Is(err, context.Canceled)
}

func (v *Verifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor *policiesv1beta1.Attestor) error {
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign image signature", "image", image.Image)

	cOpts, err := v.buildCheckOptsWithBundleDetection(ctx, attestor.Cosign, image)
	if err != nil {
		err := Setup(errors.Wrapf(err, "failed to build cosign verification opts"))
		logger.Error(err, "image verification failed")
		return err
	}

	// Set appropriate claim verifier based on format
	if cOpts.NewBundleFormat {
		cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
	} else {
		cOpts.ClaimVerifier = cosign.SimpleClaimVerifier
	}

	var sigs []oci.Signature
	var verified bool

	if cOpts.NewBundleFormat {
		sigs, verified, err = cosign.VerifyImageAttestations(ctx, image.NameRef(), cOpts)
	} else {
		sigs, verified, err = cosign.VerifyImageSignatures(ctx, image.NameRef(), cOpts)
	}
	if err != nil {
		err := classifyVerifyError(err)
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		ignoreTlog := attestor.Cosign.CTLog != nil && (attestor.Cosign.CTLog.InsecureIgnoreTlog || attestor.Cosign.CTLog.InsecureIgnoreSCT)
		if !ignoreTlog {
			err := fmt.Errorf("transparency log or timestamp verification failed")
			logger.Error(err, "image verification failed")
			return err
		}
	} else if len(sigs) == 0 {
		err := fmt.Errorf("signatures not found")
		logger.Error(err, "image verification failed")
		return err
	}

	if len(attestor.Cosign.Annotations) != 0 {
		var annotationErrors []error
		for _, sig := range sigs {
			if err := checkSignatureAnnotations(sig, attestor.Cosign.Annotations); err != nil {
				annotationErrors = append(annotationErrors, err)
				continue
			}
			return nil
		}
		err := fmt.Errorf("no signature matched the required annotations: %v", annotationErrors)
		logger.Error(err, "image verification failed")
		return err
	}

	return nil
}

func (v *Verifier) VerifyAttestationSignature(ctx context.Context, image *imagedataloader.ImageData, attestation *policiesv1beta1.Attestation, attestor *policiesv1beta1.Attestor) error {
	if attestation.InToto == nil {
		return fmt.Errorf("cosgin verifier only supports intoto referrers as attestations")
	}
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestation", attestation.Name, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign attestation signature", "image", image.Image)

	cOpts, err := v.buildCheckOptsWithBundleDetection(ctx, attestor.Cosign, image)
	if err != nil {
		err := Setup(errors.Wrapf(err, "failed to build cosign verification opts"))
		logger.Error(err, "image verification failed")
		return err
	}

	// Attestations always use IntotoSubjectClaimVerifier
	cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier

	sigs, verified, err := cosign.VerifyImageAttestations(ctx, image.NameRef(), cOpts)
	if err != nil {
		err := classifyVerifyError(err)
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		err := fmt.Errorf("cosign bundle verification failed")
		logger.Error(err, "image verification failed")
		return err
	}

	checkedTypes := []string{}
	found := false
	var annotationErrors []error
	for _, s := range sigs {
		payload, gotType, err := policy.AttestationToPayloadJSON(ctx, attestation.InToto.Type, s)
		if err != nil {
			return fmt.Errorf("converting to consumable policy validation: %w", err)
		}
		checkedTypes = append(checkedTypes, gotType)
		if len(payload) == 0 {
			// This is not the predicate type we're looking for.
			continue
		}

		if len(attestor.Cosign.Annotations) != 0 {
			if err := checkSignatureAnnotations(s, attestor.Cosign.Annotations); err != nil {
				annotationErrors = append(annotationErrors, err)
				continue
			}
		}

		found = true
		image.AddVerifiedIntotoPayloads(gotType, payload)
	}

	if !found {
		if len(annotationErrors) > 0 {
			err := fmt.Errorf("no attestation matched the required annotations: %v", annotationErrors)
			logger.Error(err, "image verification failed")
			return err
		}
		err := fmt.Errorf("required predicate type %s not found, found %v", attestation.InToto.Type, checkedTypes)
		logger.Error(err, "image verification failed")
		return err
	}

	return nil
}
