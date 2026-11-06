package cosign

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	ocistatic "github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/cosign/v3/pkg/policy"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"google.golang.org/protobuf/encoding/protojson"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type candidateStoreKey struct{}

// CandidateStore stores verified attestation candidates keyed by ImageData and predicate type.
type CandidateStore struct {
	mu         sync.Mutex
	candidates map[*imagedataloader.ImageData]map[string]verifiedCandidate
}

func NewCandidateStore() *CandidateStore {
	return &CandidateStore{
		candidates: make(map[*imagedataloader.ImageData]map[string]verifiedCandidate),
	}
}

func (s *CandidateStore) Get(image *imagedataloader.ImageData, predicateType string) (verifiedCandidate, bool) {
	if s == nil || image == nil {
		return verifiedCandidate{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.candidates == nil {
		return verifiedCandidate{}, false
	}
	if preds, ok := s.candidates[image]; ok {
		c, ok := preds[predicateType]
		return c, ok
	}
	return verifiedCandidate{}, false
}

func (s *CandidateStore) Set(image *imagedataloader.ImageData, predicateType string, candidate verifiedCandidate) {
	if s == nil || image == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.candidates == nil {
		s.candidates = make(map[*imagedataloader.ImageData]map[string]verifiedCandidate)
	}
	if s.candidates[image] == nil {
		s.candidates[image] = make(map[string]verifiedCandidate)
	}
	s.candidates[image][predicateType] = candidate
}

func (s *CandidateStore) Delete(image *imagedataloader.ImageData) {
	if s == nil || image == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.candidates, image)
}

// WithCandidateStore injects a CandidateStore into the context to scope candidate
// state to the request lifecycle.
func WithCandidateStore(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if CandidateStoreFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, candidateStoreKey{}, NewCandidateStore())
}

// CandidateStoreFromContext extracts the CandidateStore from ctx, if present.
func CandidateStoreFromContext(ctx context.Context) *CandidateStore {
	if ctx == nil {
		return nil
	}
	store, _ := ctx.Value(candidateStoreKey{}).(*CandidateStore)
	return store
}

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

func (v *Verifier) candidateStore(ctx context.Context, image *imagedataloader.ImageData) *CandidateStore {
	if store := CandidateStoreFromContext(ctx); store != nil {
		return store
	}
	return NewCandidateStore()
}

// buildCheckOptsWithBundleDetection builds CheckOpts and loads any cosign v3 bundle
// referrers attached to the image, so verification can reuse the same bounded read
// instead of asking cosign to enumerate referrers again.
func (v *Verifier) buildCheckOptsWithBundleDetection(ctx context.Context, attestor *policiesv1beta1.Cosign, image *imagedataloader.ImageData) (*cosign.CheckOpts, []*sgbundle.Bundle, *v1.Hash, error) {
	cOpts, err := checkOptions(ctx, attestor, image.RemoteOpts(), image.NameOpts(), v.secretLister)
	if err != nil {
		return nil, nil, nil, err
	}

	bundles, hash := v.loadBundles(image.NameRef(), cOpts.RegistryClientOpts)
	cOpts.NewBundleFormat = len(bundles) > 0
	if cOpts.NewBundleFormat && shouldUseSignedTimestamps(cOpts.IgnoreTlog, cOpts.UseSignedTimestamps, cOpts.TrustedMaterial) {
		cOpts.UseSignedTimestamps = true
	}

	return cOpts, bundles, hash, nil
}

// bundleLayerMediaTypePrefix identifies the layer of a cosign v3 sigstore bundle referrer.
const bundleLayerMediaTypePrefix = "application/vnd.dev.sigstore.bundle"

// loadBundles reads and parses every cosign v3 sigstore bundle referrer attached
// to the image, reproducing cosign.GetBundles's enumeration and acceptance
// predicate but through a size-bounded reader, so this is the only read any
// candidate ever gets: verification reuses these parsed bundles instead of
// asking cosign to fetch them again. A referrer that cannot be read or parsed
// within the limit is skipped, not fatal, matching cosign's own GetBundles,
// which continues past any referrer it cannot turn into a bundle.
func (v *Verifier) loadBundles(ref name.Reference, registryClientOpts []ociremote.Option) ([]*sgbundle.Bundle, *v1.Hash) {
	digest, err := ociremote.ResolveDigest(ref, registryClientOpts...)
	if err != nil {
		return nil, nil
	}
	hash, err := v1.NewHash(digest.Identifier())
	if err != nil {
		return nil, nil
	}

	index, err := ociremote.Referrers(digest, "", registryClientOpts...)
	if err != nil {
		return nil, nil
	}

	bundleRepo := digest.Repository
	if targetRepo := ociremote.TargetRepositoryFromOptions(registryClientOpts...); (targetRepo != name.Repository{}) {
		bundleRepo = targetRepo
	}

	var bundles []*sgbundle.Bundle
	for _, result := range index.Manifests {
		st, err := name.ParseReference(fmt.Sprintf("%s@%s", bundleRepo, result.Digest.String()))
		if err != nil {
			v.log.V(2).Info("skipping referrer: cannot parse referrer reference", "digest", result.Digest.String(), "err", err.Error())
			continue
		}
		// ociremote carries the attestor's source credentials and name options, so
		// the referrer manifest must be fetched through it and not through remote.
		img, err := ociremote.SignedImage(st, registryClientOpts...)
		if err != nil {
			v.log.V(2).Info("skipping referrer: cannot fetch referrer manifest", "digest", result.Digest.String(), "err", err.Error())
			continue
		}
		manifest, err := img.Manifest()
		if err != nil {
			v.log.V(2).Info("skipping referrer: cannot read referrer manifest", "digest", result.Digest.String(), "err", err.Error())
			continue
		}
		if len(manifest.Layers) != 1 {
			continue
		}
		// Cheap manifest-level filter first, so non-bundle referrers are never read.
		if !strings.HasPrefix(string(manifest.Layers[0].MediaType), bundleLayerMediaTypePrefix) {
			continue
		}
		layers, err := img.Layers()
		if err != nil || len(layers) != 1 {
			continue
		}
		bundle, err := parseBundleLayer(layers[0])
		if err != nil {
			// A referrer refused for size carries the bundle media type, so it was
			// meant to be a bundle: say so at a level the operator actually sees,
			// otherwise verification just reports "no matching signatures".
			if stderrors.Is(err, verifiers.ErrLayerTooLarge) {
				v.log.Info("skipping bundle referrer: exceeds the payload limit, raise --imageVerifyPayloadLimit to accept it", "digest", result.Digest.String(), "err", err.Error())
			} else {
				v.log.V(2).Info("skipping referrer: not a usable sigstore bundle", "digest", result.Digest.String(), "err", err.Error())
			}
			continue
		}
		bundles = append(bundles, bundle)
	}

	return bundles, &hash
}

// parseBundleLayer applies cosign ociremote.Bundle's acceptance predicate to a
// bounded read of the layer: protojson into a protobundle, sgbundle.NewBundle,
// then MinVersion v0.3. Keeping these steps identical keeps the bundle list this
// loop produces in agreement with what cosign.GetBundles would have returned.
func parseBundleLayer(layer v1.Layer) (*sgbundle.Bundle, error) {
	bundleBytes, err := verifiers.ReadLayerLimited(layer, verifiers.PayloadLimit())
	if err != nil {
		return nil, err
	}
	pb := &protobundle.Bundle{}
	if err := protojson.Unmarshal(bundleBytes, pb); err != nil {
		return nil, err
	}
	// cosign passes o.BundleOpts here; this repo never sets any, so there are none.
	b, err := sgbundle.NewBundle(pb)
	if err != nil {
		return nil, err
	}
	if !b.MinVersion("v0.3") {
		return nil, fmt.Errorf("bundle version too old (requires v0.3+)")
	}
	return b, nil
}

// verifyBundles verifies bundles already loaded by loadBundles, reproducing the
// per-bundle wrapper cosign.verifyImageAttestationsSigstoreBundle applies around
// cosign.VerifyNewBundle, but sequentially rather than through cosign's worker
// pool: require, build and claim-verify a DSSE attestation for each bundle that
// passes verification, and skip any that does not.
func verifyBundles(ctx context.Context, cOpts *cosign.CheckOpts, bundles []*sgbundle.Bundle, hash *v1.Hash) ([]oci.Signature, []*sgbundle.Bundle, []*verify.VerificationResult, bool, error) {
	// Enforce this up front, matching cosign.VerifyImageAttestations.
	if cOpts.RootCerts == nil && cOpts.SigVerifier == nil && cOpts.TrustedMaterial == nil {
		return nil, nil, nil, false, stderrors.New("one of verifier, root certs, or TrustedMaterial is required")
	}

	digestBytes, err := hex.DecodeString(hash.Hex)
	if err != nil {
		return nil, nil, nil, false, err
	}
	artifactPolicyOption := verify.WithArtifactDigest(hash.Algorithm, digestBytes)

	var checkedAttestations []oci.Signature
	var verifiedBundles []*sgbundle.Bundle
	var verifiedResults []*verify.VerificationResult
	var atLeastOneVerified bool
	var errs []error
	for _, bundle := range bundles {
		// Per-iteration copy: VerifyNewBundle's rekorV2Bundle step can write
		// UseSignedTimestamps on the CheckOpts it is given; a shared pointer would
		// let one bundle's write affect how the next bundle in this loop is verified.
		coCopy := *cOpts
		res, err := cosign.VerifyNewBundle(ctx, &coCopy, artifactPolicyOption, bundle)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		dsse, ok := bundle.Content.(*protobundle.Bundle_DsseEnvelope)
		if !ok {
			errs = append(errs, fmt.Errorf("bundle does not contain a DSSE envelope"))
			continue
		}
		payload, err := json.Marshal(dsse.DsseEnvelope)
		if err != nil {
			errs = append(errs, fmt.Errorf("marshaling DSSE envelope: %w", err))
			continue
		}
		att, err := ocistatic.NewAttestation(payload)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if cOpts.ClaimVerifier != nil {
			if err := cOpts.ClaimVerifier(att, *hash, cOpts.Annotations); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		checkedAttestations = append(checkedAttestations, att)
		verifiedBundles = append(verifiedBundles, bundle)
		verifiedResults = append(verifiedResults, res)
		atLeastOneVerified = true
	}

	if len(checkedAttestations) == 0 {
		return nil, nil, nil, false, fmt.Errorf("no matching attestations: %w", stderrors.Join(errs...))
	}

	return checkedAttestations, verifiedBundles, verifiedResults, atLeastOneVerified, nil
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

// ignoreTlog reports whether the attestor is configured to skip transparency
// log / SCT verification, e.g. for keyed/KMS attestors used in air-gapped
// setups that have no public Rekor instance to verify a bundle against.
func ignoreTlog(cosignAttestor *policiesv1beta1.Cosign) bool {
	return cosignAttestor.CTLog != nil && (cosignAttestor.CTLog.InsecureIgnoreTlog || cosignAttestor.CTLog.InsecureIgnoreSCT)
}

// verifyImageAttestationsFn indirects cosign.VerifyImageAttestations so tests
// can stub the verified=false/err=nil case without a live registry.
var verifyImageAttestationsFn = cosign.VerifyImageAttestations

func (v *Verifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor *policiesv1beta1.Attestor) error {
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign image signature", "image", image.Image)

	cOpts, bundles, hash, err := v.buildCheckOptsWithBundleDetection(ctx, attestor.Cosign, image)
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

	if len(bundles) > 0 {
		sigs, _, _, verified, err = verifyBundles(ctx, cOpts, bundles, hash)
	} else {
		sigs, verified, err = cosign.VerifyImageSignatures(ctx, image.NameRef(), cOpts)
	}
	if err != nil {
		err := errors.Wrapf(err, "failed to verify cosign signatures")
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		if !ignoreTlog(attestor.Cosign) {
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

	cOpts, bundles, hash, err := v.buildCheckOptsWithBundleDetection(ctx, attestor.Cosign, image)
	if err != nil {
		err := errors.Wrapf(err, "failed to build cosign verification opts")
		logger.Error(err, "image verification failed")
		return err
	}

	// Attestations always use IntotoSubjectClaimVerifier
	cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
	cOpts.Annotations = toAnnotationsOpt(attestor.Cosign.Annotations)

	var sigs []oci.Signature
	var verifiedBundles []*sgbundle.Bundle
	var verifiedResults []*verify.VerificationResult
	var verified bool
	if len(bundles) > 0 {
		sigs, verifiedBundles, verifiedResults, verified, err = verifyBundles(ctx, cOpts, bundles, hash)
	} else {
		sigs, verified, err = verifyImageAttestationsFn(ctx, image.NameRef(), cOpts)
	}
	if err != nil {
		err := errors.Wrapf(err, "failed to verify cosign signatures")
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		if !ignoreTlog(attestor.Cosign) {
			err := fmt.Errorf("cosign bundle verification failed")
			logger.Error(err, "image verification failed")
			return err
		}
	}

	checkedTypes := []string{}
	candidatesByPredicate := make(map[string][]verifiedCandidate)
	for i, s := range sigs {
		payload, gotType, err := policy.AttestationToPayloadJSON(ctx, attestation.InToto.Type, s)
		if err != nil {
			return fmt.Errorf("converting to consumable policy validation: %w", err)
		}
		checkedTypes = append(checkedTypes, gotType)
		if len(payload) == 0 {
			// This is not the predicate type we're looking for.
			continue
		}

		var associatedBundle *sgbundle.Bundle
		var associatedResult *verify.VerificationResult
		if i < len(verifiedBundles) {
			associatedBundle = verifiedBundles[i]
		}
		if i < len(verifiedResults) {
			associatedResult = verifiedResults[i]
		}
		bTS, hasB, sTS, hasS := extractAttestationTimestamps(s, payload, cOpts, associatedBundle, associatedResult)
		candidatesByPredicate[gotType] = append(candidatesByPredicate[gotType], verifiedCandidate{
			payload:       payload,
			predicateType: gotType,
			statementTS:   sTS,
			hasStatement:  hasS,
			bundleTS:      bTS,
			hasBundle:     hasB,
		})
	}

	if len(candidatesByPredicate) == 0 {
		err := fmt.Errorf("required predicate type %s not found, found %v", attestation.InToto.Type, checkedTypes)
		logger.Error(err, "image verification failed")
		return err
	}

	for pType, candidates := range candidatesByPredicate {
		chosen, err := selectNewestAttestation(candidates, pType)
		if err != nil {
			logger.Error(err, "image verification failed to select newest attestation")
			return err
		}

		store := v.candidateStore(ctx, image)
		if prev, ok := store.Get(image, pType); ok {
			if bytes.Equal(prev.payload, chosen.payload) {
				// Byte-identical payload already stored across attestors/evaluations;
				// preserve whichever candidate has verified timestamps.
				if (!prev.hasBundle && chosen.hasBundle) || (!prev.hasStatement && chosen.hasStatement) {
					store.Set(image, pType, chosen)
				}
				image.AddVerifiedIntotoPayloads(chosen.predicateType, chosen.payload)
				continue
			}
			merged, err := selectNewestAttestation([]verifiedCandidate{prev, chosen}, pType)
			if err != nil {
				logger.Error(err, "image verification failed to merge newest attestation across attestors")
				return err
			}
			chosen = merged
		}
		store.Set(image, pType, chosen)

		image.AddVerifiedIntotoPayloads(chosen.predicateType, chosen.payload)
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
