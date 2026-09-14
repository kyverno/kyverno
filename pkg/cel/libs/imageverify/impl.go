package imageverify

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	signatureCacheRule   = "verifyImageSignatures"
	attestationCacheRule = "verifyAttestationSignatures"
)

type IvFuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext
	policy          v1beta1.ImageValidatingPolicyLike
	creds           *v1beta1.Credentials
	imgRules        []compiler.MatchImageReference
	attestationList map[string]v1beta1.Attestation
	cosignVerifier  cosignImageVerifier
	notaryVerifier  *notary.Verifier
	ivCache         imageverifycache.Client
	authOpts        []remote.Option
	nameOpts        []name.Option
	verifications   *ImageVerificationResults
	diagnostics     *verificationDiagnostics

	// requestCtx is the admission request's context. CEL function bindings only
	// receive their arguments, so the per-request copy made by
	// NewRuntimeForPolicy carries it.
	requestCtx context.Context //nolint:containedctx

	// pendingAttestationRestores holds verified attestation payloads (InToto
	// predicates or Notary/OCI-referrer artifacts) read back from the cache on
	// a verifyAttestationSignatures() hit, keyed by "<image>\x00<attestation>".
	// Applying them to ImageData is deferred until extractPayload() actually
	// asks for that image+attestation, so a policy that only calls
	// verifyAttestationSignatures() never pays for it.
	//
	// Every attestation type is required to have a cached payload before a
	// cache hit is trusted (see verify_image_attestations_string_string_stringarray):
	// for InToto, ImageData.GetPayload errors out without one; for
	// Referrer/Notary it has no such guard and will silently fetch an
	// unverified artifact from the registry instead (see #17130), so the
	// verify-side degrade check must never special-case InToto only.
	//
	// The cache *write* on a miss stays eager (see
	// verify_image_attestations_string_string_stringarray): img is already
	// in memory at that point, so caching the payload costs no extra I/O,
	// and deferring it to extractPayload() would mean a verify-only policy
	// never completes the write -- so it would never get a real cache hit
	// again, defeating caching entirely for that common case.
	pendingAttestationRestores map[string]map[string][]byte
}

// cosignImageVerifier exists only so tests can swap in a fake verifier;
// production always uses *cosign.Verifier.
type cosignImageVerifier interface {
	VerifyImageSignature(context.Context, *imagedataloader.ImageData, *v1beta1.Attestor) error
	VerifyAttestationSignature(context.Context, *imagedataloader.ImageData, *v1beta1.Attestation, *v1beta1.Attestor) error
}

// Runtime holds state owned by a single request. NewRuntimeForPolicy creates a
// policy-local function implementation, so deferred payload restoration is never shared.
type Runtime struct {
	functions *IvFuncs
}

// NewRuntimeForPolicy copies the policy's IvFuncs and attaches request-owned
// state to the copy. This is needed because the same compiled policy can be used
// by two admission requests, and those shouldn't replace stateful fields in IvFuncs
// that belong to eachother
func NewRuntimeForPolicy(ctx context.Context, f *IvFuncs, imgCtx imagedataloader.ImageContext, cache imageverifycache.Client, results *ImageVerificationResults) Runtime {
	newFuncs := *f
	newFuncs.requestCtx = ctx
	newFuncs.imgCtx = imgCtx
	newFuncs.ivCache = cache
	newFuncs.verifications = results
	newFuncs.pendingAttestationRestores = map[string]map[string][]byte{}
	newFuncs.diagnostics = &verificationDiagnostics{}

	return Runtime{functions: &newFuncs}
}

// requestContext returns the admission request's context, so registry and
// verification work stops when the request is cancelled or times out.
func (f *IvFuncs) requestContext() context.Context {
	if f.requestCtx != nil {
		return f.requestCtx
	}
	return context.Background()
}

// WithRequestContext returns opts with remote.WithContext(ctx) appended last.
// go-containerregistry applies options in order, so this overrides the context
// that registryclient.GlobalOptsOrDefault puts into the options when a policy is
// compiled. The slice is clipped first because compiled policies share their
// options across admission requests.
func WithRequestContext(ctx context.Context, opts []remote.Option) []remote.Option {
	return append(slices.Clip(opts), remote.WithContext(ctx))
}

func NewIvFuncs(
	logger logr.Logger,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister corev1listers.SecretLister,
	adapter types.Adapter,
	imgRules []compiler.MatchImageReference,
) *IvFuncs {
	spec := ivpol.GetSpec()

	// by default, try to use the options built globally from flags
	authOpts, nameOpts := registryclient.GlobalOptsOrDefault(context.Background())
	if spec.Credentials != nil {
		authOpts, nameOpts = regcreds.RemoteOptsFromIvpolCredentials(lister, *spec.Credentials, config.KyvernoNamespace(), logger)
	}

	return &IvFuncs{
		Adapter:         adapter,
		logger:          logger,
		policy:          ivpol,
		creds:           spec.Credentials,
		imgRules:        imgRules,
		attestationList: attestationMap(ivpol),
		nameOpts:        nameOpts,
		authOpts:        authOpts,
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
	}
}

// build a cache key from a CEL function name, a qualifier (attestation name in practice)
// and the sorted group of attestors
func attestorCacheRule(fn string, qualifier string, attestors []v1beta1.Attestor) string {
	names := make([]string, 0, len(attestors))
	for _, attestor := range attestors {
		names = append(names, attestor.GetKey())
	}
	sort.Strings(names)
	var b strings.Builder
	writeCacheKeyPart(&b, fn)
	writeCacheKeyPart(&b, qualifier)
	for _, name := range names {
		writeCacheKeyPart(&b, name)
	}
	return b.String()
}

func writeCacheKeyPart(b *strings.Builder, part string) {
	fmt.Fprintf(b, "%d:%s|", len(part), part)
}

// pendingKey builds the request-scoped lookup key used by the
// pendingAttestationRestores map.
func pendingKey(image, attestation string) string {
	return image + "\x00" + attestation
}

func (f *IvFuncs) verify_image_signature_string_stringarray(image ref.Val, attestors ref.Val) ref.Val {
	ctx := f.requestContext()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else if attestors, err := utils.ConvertToNative[[]v1beta1.Attestor](attestors); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		if match, err := matching.MatchImage(image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			f.logger.V(4).Info("skipping image, no matchImageReferences match", "image", image)
			return f.NativeToValue(count)
		}
		f.logger.V(4).Info("verifyImageSignatures called", "image", image, "attestorCount", len(attestors))

		// create a rule with the given attestors
		cacheRule := attestorCacheRule(signatureCacheRule, "", attestors)
		if f.ivCache != nil {
			if found, err := f.ivCache.Get(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				f.logger.V(4).Info("image signature verification cache hit", "image", image, "policy", f.policy.GetName())
				f.verifications.Record(image, true)
				return f.NativeToValue(len(attestors))
			}
		}

		// Fetch image data once before the loop: the image reference and
		// credentials are the same for every attestor.
		img, err := f.imgCtx.Get(ctx, image, WithRequestContext(ctx, f.authOpts), f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for _, attestor := range attestors {
			if attestor.IsCosign() {
				f.logger.V(4).Info("verifying image signature", "image", image, "attestor", attestor.Name, "type", "cosign")
				if err := f.cosignVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
					f.diagnostics.record(image, attestor.Name, "", err)
					f.logger.V(6).Info("image signature verification failed", "image", image, "attestor", attestor.Name, "type", "cosign", "error", err)
				} else {
					f.logger.V(4).Info("image signature verified", "image", image, "attestor", attestor.Name, "type", "cosign")
					count += 1
				}
			} else if attestor.IsNotary() {
				var certs, tsaCerts string
				if attestor.Notary.Certs != nil {
					certs = attestor.Notary.Certs.Value
				}
				if attestor.Notary.TSACerts != nil {
					tsaCerts = attestor.Notary.TSACerts.Value
				}
				f.logger.V(4).Info("verifying image signature", "image", image, "attestor", attestor.Name, "type", "notary")
				if err := f.notaryVerifier.VerifyImageSignature(ctx, img, certs, tsaCerts); err != nil {
					f.diagnostics.record(image, attestor.Name, "", err)
					f.logger.V(6).Info("image signature verification failed", "image", image, "attestor", attestor.Name, "type", "notary", "error", err)
				} else {
					f.logger.V(4).Info("image signature verified", "image", image, "attestor", attestor.Name, "type", "notary")
					count += 1
				}
			}
		}
		f.logger.V(6).Info("verifyImageSignatures returning", "image", image, "verifiedCount", count)
		if f.ivCache != nil && len(attestors) > 0 && count == len(attestors) {
			if _, err := f.ivCache.Set(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache set", "image", image)
			}
		}
		if len(attestors) > 0 {
			f.verifications.Record(image, count > 0)
		}
		return f.NativeToValue(count)
	}
}

func (f *IvFuncs) verify_image_attestations_string_string_stringarray(args ...ref.Val) ref.Val {
	ctx := f.requestContext()
	if len(args) != 3 {
		return types.NewErr("function usage: <image> <attestation> <attestor list>")
	}
	if image, err := utils.ConvertToNative[string](args[0]); err != nil {
		return types.WrapErr(err)
	} else if attestation, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if attestors, err := utils.ConvertToNative[[]v1beta1.Attestor](args[2]); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		if match, err := matching.MatchImage(image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			f.logger.V(4).Info("skipping image, no matchImageReferences match", "image", image)
			return f.NativeToValue(count)
		}
		f.logger.V(4).Info("verifyAttestationSignatures called", "image", image, "attestation", attestation, "attestorCount", len(attestors))
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}
		cacheRule := attestorCacheRule(attestationCacheRule, attestation, attestors)
		if f.ivCache != nil {
			if found, payloads, err := f.ivCache.GetWithPayload(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				payloadKey := attestationPayloadKey(attest)
				if payloadKey == "" || len(payloads[payloadKey]) == 0 {
					// A degraded entry (cached "found" but no payload for
					// this attestation's specific predicate/artifact type to
					// restore) can't be trusted as a hit for ANY attestation
					// type -- we can't safely defer this decision since we
					// don't know yet whether extractPayload() will be called
					// later in this same evaluation. This matters even more
					// for Referrer/Notary attestations than InToto: InToto's
					// GetPayload errors out with nothing to restore, but
					// Referrer/Notary's has no such guard and silently falls
					// back to fetching an unverified artifact straight from
					// the registry instead (see #17130). Fall back to full
					// re-verification below rather than denying an admission
					// that was already verified once, or -- worse --
					// returning unverified data as if it were verified.
					f.logger.V(4).Info("cache hit has no payload to restore, falling back to re-verification", "image", image, "attestation", attestation)
				} else {
					// Defer applying the payload to ImageData (which needs an
					// imgCtx.Get()) until extractPayload() actually asks for
					// it -- a verify-only policy never pays for it.
					f.pendingAttestationRestores[pendingKey(image, attestation)] = payloads
					f.logger.V(4).Info("image attestation verification cache hit", "image", image, "policy", f.policy.GetName())
					f.verifications.Record(image, true)
					return f.NativeToValue(len(attestors))
				}
			}
		}
		img, err := f.imgCtx.Get(ctx, image, WithRequestContext(ctx, f.authOpts), f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for _, attestor := range attestors {
			if attestor.IsCosign() {
				f.logger.V(4).Info("verifying attestation signature", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "cosign")
				if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
					f.diagnostics.record(image, attestor.Name, attestation, err)
					f.logger.V(6).Info("attestation signature verification failed", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "cosign", "error", err)
				} else {
					f.logger.V(4).Info("attestation signature verified", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "cosign")
					count += 1
				}
			} else if attestor.IsNotary() {
				if attest.Referrer == nil {
					return types.NewErr("notary verifier only supports oci 1.1 referrers as attestations")
				}
				var certs, tsaCerts string
				if attestor.Notary.Certs != nil {
					certs = attestor.Notary.Certs.Value
				}
				if attestor.Notary.TSACerts != nil {
					tsaCerts = attestor.Notary.TSACerts.Value
				}
				f.logger.V(4).Info("verifying attestation signature", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary")
				if err := f.notaryVerifier.VerifyAttestationSignature(ctx, img, attest.Referrer.Type, certs, tsaCerts); err != nil {
					f.diagnostics.record(image, attestor.Name, attestation, err)
					f.logger.V(6).Info("attestation signature verification failed", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary", "error", err)
				} else {
					f.logger.V(4).Info("attestation signature verified", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary")
					count += 1
				}
			}
		}
		f.logger.V(6).Info("verifyAttestationSignatures returning", "image", image, "attestation", attestation, "verifiedCount", count)
		if f.ivCache != nil && len(attestors) > 0 && count == len(attestors) {
			// The write stays eager: img is already in memory (fetched
			// above for verification), so extracting the payload here costs
			// no extra I/O -- unlike the read-side restore, which does.
			// Deferring this write to extractPayload() would mean a
			// verify-only policy (which never calls it) never completes
			// the write, so it would never get a real cache hit again.
			payloads := attestationPayloadFromImage(img, attest)
			if len(payloads) == 0 {
				// Verification succeeded but we couldn't capture the payload to
				// cache alongside it. Skip the cache write entirely rather than
				// recording a presence-only hit: a future admission would see
				// "found" but have nothing to restore, which -- depending on
				// attestation type -- either reproduces the original
				// "cannot be fetch before verifying" error (InToto) or, worse,
				// silently falls back to an unverified registry fetch
				// (Referrer/Notary, see #17130). Leaving this uncached means
				// the next request re-verifies from scratch instead of
				// degrading either way.
				f.logger.Error(nil, "skipping cache write: failed to capture attestation payload after successful verification", "image", image, "attestation", attestation)
			} else if stored, err := f.ivCache.SetWithPayload(ctx, f.policy, cacheRule, image, true, payloads); err != nil {
				f.logger.Error(err, "error occurred during image verify cache set", "image", image)
			} else if !stored && f.ivCache.Enabled() {
				// SetWithPayload also returns false when caching is disabled entirely,
				// which isn't a rejection worth logging -- Enabled() rules that out.
				// What's left is the underlying cache actually declining the write,
				// e.g. because the payload's byte cost exceeds --imageVerifyCacheMaxSize.
				// That's safe -- GetWithPayload will simply miss and this image
				// re-verifies from scratch -- but silent by default, so surface it
				// rather than let caching effectiveness degrade unnoticed. See
				// kyverno/kyverno#17575 for the sizing follow-up.
				f.logger.Error(nil, "cache write rejected: attestation payload was not cached, possibly exceeding --imageVerifyCacheMaxSize; this image will re-verify from scratch on the next admission", "image", image, "attestation", attestation)
			}
		}
		if len(attestors) > 0 {
			f.verifications.Record(image, count > 0)
		}
		return f.NativeToValue(count)
	}
}

// attestationPayloadKey returns the map key a verified attestation's payload
// is cached and restored under: the InToto predicate type or the
// Referrer/Notary artifact type, whichever the attestation carries.
func attestationPayloadKey(attest v1beta1.Attestation) string {
	if attest.IsInToto() {
		return attest.InToto.Type
	}
	if attest.IsReferrer() {
		return attest.Referrer.Type
	}
	return ""
}

// attestationPayloadFromImage reads the verified attestation payload (an
// InToto predicate or a Notary/OCI-referrer artifact) from ImageData after a
// successful Cosign or Notary attestation verify. ImageData does not expose a
// getter for the raw verifiedIntotoPayloads/verifiedReferrers maps, so we
// round-trip through GetPayload + json.Marshal instead.
//
// This must only be called immediately after a verification that populated
// img with THIS request's verified data. Calling it on a fresh/restored
// ImageData -- e.g. one that never went through verification this request --
// would hit GetPayload's Referrer/Notary fallback path, which fetches an
// unverified artifact straight from the registry (see #17130).
//
// Safe degrade: if GetPayload (or Marshal) fails, we return nil and the
// caller skips caching this result entirely rather than recording a
// presence-only entry.
func attestationPayloadFromImage(img *imagedataloader.ImageData, attest v1beta1.Attestation) map[string][]byte {
	key := attestationPayloadKey(attest)
	if img == nil || key == "" {
		return nil
	}
	payload, err := img.GetPayload(attest)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return map[string][]byte{key: b}
}

func (f *IvFuncs) payload_string_string(image ref.Val, attestation ref.Val) ref.Val {
	ctx := f.requestContext()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else if attestation, err := utils.ConvertToNative[string](attestation); err != nil {
		return types.WrapErr(err)
	} else {
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}
		key := pendingKey(image, attestation)

		// Referrer/Notary payloads are served directly from the cache-hit
		// restore, on every call, and never through ImageData.GetPayload():
		// the SDK has no setter to record a verified referrer from raw
		// bytes, and calling GetPayload() on an ImageData with nothing
		// recorded silently falls back to fetching an unverified artifact
		// from the registry (see #17130). Unlike InToto's restore, this
		// entry is intentionally NOT deleted after one use -- a policy
		// commonly calls extractPayload() more than once for the same
		// image+attestation (e.g. checking several payload fields across
		// separate CEL expressions), and every one of those calls must stay
		// on the verified path, not just the first. A cache entry missing
		// the specific artifact type this attestation expects is treated as
		// a hard failure here too, matching InToto's fail-closed behavior,
		// rather than falling through to the unverified live fetch.
		if attest.IsReferrer() {
			if payloads, ok := f.pendingAttestationRestores[key]; ok {
				data, ok := payloads[attest.Referrer.Type]
				if !ok {
					return types.NewErr("cached attestation payload for %q is missing verified artifact type %q", attestation, attest.Referrer.Type)
				}
				var payload any
				if err := json.Unmarshal(data, &payload); err != nil {
					return types.NewErr("failed to unmarshal cached attestation payload: %v", err)
				}
				return f.NativeToValue(payload)
			}
		}

		img, err := f.imgCtx.Get(ctx, image, WithRequestContext(ctx, f.authOpts), f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		// Complete a deferred InToto restore from a same-request cache hit:
		// only now, since extractPayload() was actually called, apply the
		// cached payload to this fresh ImageData. AddVerifiedIntotoPayloads
		// makes this permanent on img, so repeat extractPayload() calls for
		// the same image+attestation stay safe via GetPayload() below
		// without needing this map entry again -- unlike Referrer, which has
		// no such permanent-on-img equivalent.
		if attest.IsInToto() {
			if payloads, ok := f.pendingAttestationRestores[key]; ok {
				delete(f.pendingAttestationRestores, key)
				if data, ok := payloads[attest.InToto.Type]; ok {
					img.AddVerifiedIntotoPayloads(attest.InToto.Type, data)
				}
			}
		}

		payload, err := img.GetPayload(attest)
		if err != nil {
			return types.NewErr("failed to get payload: %v", err)
		}

		return f.NativeToValue(payload)
	}
}

func (f *IvFuncs) get_image_data_string(image ref.Val) ref.Val {
	ctx := f.requestContext()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else {
		img, err := f.imgCtx.Get(ctx, image, WithRequestContext(ctx, f.authOpts), f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		// Convert through JSON: since cel-go v0.31 (#17067) NativeToValue only converts
		// registered native types, and imagedataloader.ImageData is not one.
		data, err := utils.GetValue(img.Data())
		if err != nil {
			return types.NewErr("failed to convert imagedata: %v", err)
		}
		return f.NativeToValue(data)
	}
}
