package cosign

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
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

	// Auto-detect if new bundle format (cosign v3) is actually present
	bundleDetected, err := hasSigstoreBundles(image, cOpts)
	if err != nil {
		// Discovery that cannot complete leaves the legacy path, exactly as the
		// cosign.GetBundles call this replaced did by folding any error into
		// "no bundles found".
		v.log.V(4).Info("bundle discovery failed, assuming legacy format", "image", image.Image, "error", err)
		bundleDetected = false
	}
	cOpts.NewBundleFormat = bundleDetected
	if bundleDetected && shouldUseSignedTimestamps(cOpts.IgnoreTlog, cOpts.UseSignedTimestamps, cOpts.TrustedMaterial) {
		cOpts.UseSignedTimestamps = true
	}

	return cOpts, nil
}

// bundleMediaTypePrefix is the media type cosign gives a sigstore bundle, both
// as the artifactType of the referrer manifest and as the media type of the
// layer holding the bundle itself (pkg/oci/remote/write.go).
const bundleMediaTypePrefix = "application/vnd.dev.sigstore.bundle"

// isBundleMediaType reports whether a media type names a sigstore bundle that
// cosign would go on to verify, which means format v0.3 or later.
//
// The version check matters: ociremote.Bundle rejects anything older (it parses
// the blob and calls MinVersion("v0.3")), so cosign.GetBundles would not count a
// v0.1/v0.2 bundle and verification stays on the legacy path. Treating one as
// new-format here would instead route it to VerifyImageAttestations, which then
// fails with "no valid bundles exist in registry" -- turning a working fallback
// into a denial. Those two versions carry their version as a media type
// parameter; v0.3 onwards spell it in the subtype, so anything else matching the
// prefix is v0.3 or later.
func isBundleMediaType(mediaType string) bool {
	switch mediaType {
	case bundleMediaTypePrefix + "+json;version=0.1", bundleMediaTypePrefix + "+json;version=0.2":
		return false
	}
	return strings.HasPrefix(mediaType, bundleMediaTypePrefix)
}

// hasSigstoreBundles reports whether the image carries at least one cosign v3
// sigstore bundle, reading the referrers index and, only where that is not
// conclusive, the referrer manifests -- never the bundle blobs.
//
// cosign.GetBundles answers the same question, but to do so it fetches and
// parses the blob behind every referrer (ociremote.Bundle reads the layer), and
// the answer is all that is kept: VerifyImageAttestations calls GetBundles again
// to do the actual verification. On an image carrying large attestations -- an
// SBOM, a vulnerability report -- that makes every bundle a download that
// happens twice per check, to decide a boolean.
//
// The index settles it alone when an entry carries the bundle media type as its
// artifactType, which is how a registry serving the referrers API reports the
// manifests cosign writes. On a registry without one, go-containerregistry falls
// back to the sha256-<digest> tag, an index maintained by whichever client wrote
// the referrer, and some type the entry from the manifest's config, so it reads
// only application/vnd.oci.empty.v1+json (kyverno#16664). Those are settled by
// reading the referrer manifest, which is under a kilobyte and carries the layer
// media type ociremote.Bundle itself screens on.
func hasSigstoreBundles(img *imagedataloader.ImageData, cOpts *cosign.CheckOpts) (bool, error) {
	// img.Digest is already resolved, so unlike GetBundles this needs no
	// ResolveDigest round trip to turn a tag into a digest.
	digest := img.NameRef().Context().Digest(img.Digest)
	index, err := ociremote.Referrers(digest, "", cOpts.RegistryClientOpts...)
	if err != nil {
		return false, err
	}

	// bundles live in the signature repository when the attestor configures one,
	// which is how GetBundles resolves them
	bundleRepo := digest.Repository
	if target := ociremote.TargetRepositoryFromOptions(cOpts.RegistryClientOpts...); (target != name.Repository{}) {
		bundleRepo = target
	}

	// first pass over the index alone: no requests at all
	for _, desc := range index.Manifests {
		if isBundleMediaType(desc.ArtifactType) {
			return true, nil
		}
	}

	// nothing conclusive in the index, so read the referrer manifests
	for _, desc := range index.Manifests {
		// parsed without the image's name options, as GetBundles parses it when
		// VerifyImageAttestations calls it without any: with name.Insecure this
		// would read a plain-HTTP registry that verification then reads over HTTPS
		ref, err := name.ParseReference(fmt.Sprintf("%s@%s", bundleRepo, desc.Digest.String()))
		if err != nil {
			return false, err
		}
		// SignedImage fetches the manifest; layers stay lazy, so no blob is read
		bundleImage, err := ociremote.SignedImage(ref, cOpts.RegistryClientOpts...)
		if err != nil {
			// a referrer that cannot be read is not a bundle we could verify, and
			// GetBundles skips these rather than failing discovery
			continue
		}
		manifest, err := bundleImage.Manifest()
		if err != nil {
			continue
		}
		// ociremote.Bundle requires exactly one layer and screens its media type
		if len(manifest.Layers) == 1 && isBundleMediaType(string(manifest.Layers[0].MediaType)) {
			return true, nil
		}
	}

	return false, nil
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

func (v *Verifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor *policiesv1beta1.Attestor) error {
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign image signature", "image", image.Image)

	cOpts, err := v.buildCheckOptsWithBundleDetection(ctx, attestor.Cosign, image)
	if err != nil {
		err := errors.Wrapf(err, "failed to build cosign verification opts")
		logger.Error(err, "image verification failed")
		return err
	}

	// Set appropriate claim verifier based on format
	if cOpts.NewBundleFormat {
		// cosign checks these against the subject annotations of each bundle it verified
		cOpts.Annotations = toAnnotationsOpt(attestor.Cosign.Annotations)
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
		err := errors.Wrapf(err, "failed to verify cosign signatures")
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

	if len(attestor.Cosign.Annotations) != 0 && !cOpts.NewBundleFormat {
		var annotationErrors []error
		for _, sig := range sigs {
			if err := checkSignatureAnnotationsV2(sig, attestor.Cosign.Annotations); err != nil {
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
		err := errors.Wrapf(err, "failed to build cosign verification opts")
		logger.Error(err, "image verification failed")
		return err
	}

	// Attestations always use IntotoSubjectClaimVerifier
	cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
	cOpts.Annotations = toAnnotationsOpt(attestor.Cosign.Annotations)

	sigs, verified, err := cosign.VerifyImageAttestations(ctx, image.NameRef(), cOpts)
	if err != nil {
		err := errors.Wrapf(err, "failed to verify cosign signatures")
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		err := fmt.Errorf("cosign bundle verification failed")
		logger.Error(err, "image verification failed")
		return err
	}

	checkedTypes := []string{}
	found := false
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

		found = true
		image.AddVerifiedIntotoPayloads(gotType, payload)
	}

	if !found {
		err := fmt.Errorf("required predicate type %s not found, found %v", attestation.InToto.Type, checkedTypes)
		logger.Error(err, "image verification failed")
		return err
	}

	return nil
}

func toAnnotationsOpt(annotations map[string]string) map[string]interface{} {
	if len(annotations) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(annotations))
	for k, v := range annotations {
		out[k] = v
	}
	return out
}
