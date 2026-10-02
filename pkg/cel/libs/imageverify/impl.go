package imageverify

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	imagecredentials "github.com/kyverno/kyverno/pkg/image/verification/credentials"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	notaryVerifier  notaryImageVerifier
	ivCache         imageverifycache.Client
	authOpts        []remote.Option
	nameOpts        []name.Option
	verifications   *ImageVerificationResults
	diagnostics     *verificationDiagnostics

	// requestCtx is the admission request's context. CEL function bindings only
	// receive their arguments, so the per-request copy made by
	// NewRuntimeForPolicy carries it.
	requestCtx context.Context //nolint:containedctx

	// when policies have a call to verifyAttestationSignatures we won't ever make
	// a registry call, hence get no ImageData. if the same policy then later calls
	// extractPayload(), then we get a bunch of attestations we aren't sure are signed
	// by the attestor in the policy. the ivCache however stores whatever signed attestations
	// we saw within its ttl period. this field stores them within the context of a single
	// admission event so we can push them to the ImageData type
	pendingIntotoRestores map[string]map[string][]byte

	// same, but for referrer attestors
	pendingReferrerRestores map[string][]gcrv1.Descriptor
}

// cosignImageVerifier exists only so tests can swap in a fake verifier;
// production always uses *cosign.Verifier.
type cosignImageVerifier interface {
	VerifyImageSignature(context.Context, *imagedataloader.ImageData, *v1beta1.Attestor) error
	VerifyAttestationSignature(context.Context, *imagedataloader.ImageData, *v1beta1.Attestation, *v1beta1.Attestor) error
}

// notaryImageVerifier exists only so tests can swap in a fake verifier;
// production always uses *notary.Verifier.
type notaryImageVerifier interface {
	VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, certsData, tsaCertsData string) error
	VerifyAttestationSignature(ctx context.Context, image *imagedataloader.ImageData, referrerType, certsData, tsaCertsData string) ([]gcrv1.Descriptor, error)
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
	newFuncs.requestCtx = cosign.WithCandidateStore(ctx)
	newFuncs.imgCtx = imgCtx
	newFuncs.ivCache = cache
	newFuncs.verifications = results
	newFuncs.authOpts = reuseRegistryAuth(f.authOpts, f.logger)
	newFuncs.pendingIntotoRestores = map[string]map[string][]byte{}
	newFuncs.pendingReferrerRestores = map[string][]gcrv1.Descriptor{}
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

// AuthOpts returns the registry options bound to this evaluation, including the
// shared puller added by NewRuntimeForPolicy. Callers that fetch image data for
// the same policy and request should use these rather than the options the policy
// was compiled with, so the whole evaluation goes through one puller. It returns
// nil for a zero Runtime.
func (r Runtime) AuthOpts() []remote.Option {
	if r.functions == nil {
		return nil
	}
	return r.functions.authOpts
}

// reuseRegistryAuth appends a puller to authOpts so that every registry call made
// while evaluating one policy against one request shares a single auth handshake
// per repository. Without it go-containerregistry builds a fetcher per call, and
// each one re-runs the /v2/ ping and token exchange, which dominates the time of
// a check against a registry using token auth.
//
// The puller is built in NewRuntimeForPolicy and not in NewIvFuncs: the IvFuncs a
// policy is compiled with outlives the request, while a puller caches the token
// it obtained for a repository. Keeping it request-scoped is what preserves the
// credential freshness that registryclient.GlobalOptsOrDefault protects by
// deliberately not handing out a global puller, and that
// TestFactoryCredentialsObserveSecretRotation covers.
//
// Each NewRuntimeForPolicy builds its own puller from the options it was handed,
// so two evaluations never share one.
//
// That alone does not isolate policies carrying different spec.Credentials: the
// ImageContext cache is keyed by image reference only, so the first policy to
// fetch an image fixes the options -- keychain as much as puller -- recorded on
// the ImageData that every later policy reuses for it. That sharing predates
// this change and is not widened by it: the cached keychain already resolved the
// first policy's credentials for those calls, so reusing its token alongside
// grants no access the keychain did not already grant.
func reuseRegistryAuth(authOpts []remote.Option, logger logr.Logger) []remote.Option {
	puller, err := remote.NewPuller(authOpts...)
	if err != nil {
		// reuse is an optimisation, so fall back to the unpooled options rather
		// than failing an evaluation that would otherwise have succeeded.
		logger.V(4).Info("failed to build registry puller, continuing without auth reuse", "error", err)
		return authOpts
	}
	// copied rather than appended in place: authOpts belongs to the policy's
	// IvFuncs and is shared by every runtime created from it.
	bound := make([]remote.Option, 0, len(authOpts)+1)
	bound = append(bound, authOpts...)
	return append(bound, remote.Reuse(puller))
}

func NewIvFuncs(
	logger logr.Logger,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister corev1listers.SecretLister,
	adapter types.Adapter,
	imgRules []compiler.MatchImageReference,
) (*IvFuncs, error) {
	ivpol, scopeErrs := imagecredentials.ScopePolicy(ivpol)
	if len(scopeErrs) != 0 {
		return nil, scopeErrs.ToAggregate()
	}
	lister = kubeutils.ScopeSecretLister(lister, ivpol.GetNamespace())
	// Resolve bare Secret names in the policy's own namespace; only a
	// cluster-scoped policy falls back to Kyverno's. See compiler.go for why
	// this is defence in depth rather than the thing correctness rests on.
	credentialNamespace := kubeutils.CredentialNamespace(lister, config.KyvernoNamespace())
	spec := ivpol.GetSpec()

	// by default, try to use the options built globally from flags
	authOpts, nameOpts := registryclient.GlobalOptsOrDefault(context.Background())
	if spec.Credentials != nil {
		authOpts, nameOpts = imagecredentials.RemoteOptions(lister, *spec.Credentials, credentialNamespace, logger)
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
	}, nil
}

// pendingKey builds the request-scoped lookup key used by the
// pendingIntotoRestores map.
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
		attestors, scopeErrs := imagecredentials.ScopeAttestors(f.policy, attestors)
		if len(scopeErrs) != 0 {
			return types.WrapErr(scopeErrs.ToAggregate())
		}
		count := 0
		if match, err := matching.MatchImage(ctx, image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			f.logger.V(4).Info("skipping image, no matchImageReferences match", "image", image)
			return f.NativeToValue(count)
		}
		f.logger.V(4).Info("verifyImageSignatures called", "image", image, "attestorCount", len(attestors))

		if f.ivCache != nil {
			if found, _, err := f.ivCache.Get(image, attestors, nil); err != nil {
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
			if _, err := f.ivCache.Set(image, attestors, nil, nil); err != nil {
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
		attestors, scopeErrs := imagecredentials.ScopeAttestors(f.policy, attestors)
		if len(scopeErrs) != 0 {
			return types.WrapErr(scopeErrs.ToAggregate())
		}
		count := 0
		if match, err := matching.MatchImage(ctx, image, f.imgRules...); err != nil {
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
		if f.ivCache != nil {
			if found, payloads, err := f.ivCache.Get(image, attestors, &attest); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				usable := len(payloads) > 0
				var referrers []gcrv1.Descriptor
				if attest.IsReferrer() {
					referrers, usable = decodeVerifiedReferrers(payloads[attest.Referrer.Type], attest.Referrer.Type)
				}
				if !usable {
					// there are verified payloads in the cache but they are unusable. fetch the image from the start
					f.logger.V(4).Info("cache hit has no payload to restore, falling back to re-verification", "image", image, "attestation", attestation)
				} else {
					// store payloads in case a subsequent call to extractPayload is gonna
					// happen in that admission request
					if len(referrers) > 0 {
						f.pendingReferrerRestores[pendingKey(image, attestation)] = referrers
					} else if len(payloads) > 0 {
						f.pendingIntotoRestores[pendingKey(image, attestation)] = payloads
					}
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

		var verifiedReferrers []gcrv1.Descriptor
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
				if verified, err := f.notaryVerifier.VerifyAttestationSignature(ctx, img, attest.Referrer.Type, certs, tsaCerts); err != nil {
					f.diagnostics.record(image, attestor.Name, attestation, err)
					f.logger.V(6).Info("attestation signature verification failed", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary", "error", err)
				} else {
					f.logger.V(4).Info("attestation signature verified", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary")
					verifiedReferrers = append(verifiedReferrers, verified...)
					count += 1
				}
			}
		}
		f.logger.V(6).Info("verifyAttestationSignatures returning", "image", image, "attestation", attestation, "verifiedCount", count)
		if f.ivCache != nil && len(attestors) > 0 && count == len(attestors) {
			// extract verified payloads (attestations) from the verified image
			// so we can put it in the ivCache
			payloads := intotoPayloadsFromImage(img, attest)
			if attest.IsReferrer() {
				payloads = verifiedReferrersPayload(attest, verifiedReferrers)
			}
			if len(payloads) == 0 {
				// we couldn't capture payloads. don't set anything in the cache. the next admission request
				// will verify from scratch
				f.logger.Error(nil, "skipping cache write: failed to capture attestation payload after successful verification", "image", image, "attestation", attestation)
			} else if _, err := f.ivCache.Set(image, attestors, &attest, payloads); err != nil {
				f.logger.Error(err, "error occurred during image verify cache set", "image", image)
			}
		}
		if len(attestors) > 0 {
			f.verifications.Record(image, count > 0)
		}
		return f.NativeToValue(count)
	}
}

// intotoPayloadsFromImage reads verified intoto payloads from ImageData after a
// successful Cosign attestation verify. ImageData does not expose a getter for
// the raw map, so we round-trip through GetPayload + json.Marshal.
//
// Safe degrade: if GetPayload (or Marshal) fails, we return nil and the
// caller skips caching this result entirely rather than recording a
// presence-only entry.
func intotoPayloadsFromImage(img *imagedataloader.ImageData, attest v1beta1.Attestation) map[string][]byte {
	if img == nil || !attest.IsInToto() || attest.InToto == nil {
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
	return map[string][]byte{attest.InToto.Type: b}
}

// verifiedReferrersPayload caches the descriptors of the referrers Notary
// verified, not their content: extractPayload() restores them onto ImageData,
// which fetches the content by digest. It is a list so every verified referrer
// of a type can be cached (#17892). Each referrer is cached once, and only with
// the fields the restore needs, so registry-supplied annotations or embedded
// data can't inflate the entry.
func verifiedReferrersPayload(attest v1beta1.Attestation, verified []gcrv1.Descriptor) map[string][]byte {
	var descs []gcrv1.Descriptor
	for _, d := range verified {
		if !slices.ContainsFunc(descs, func(e gcrv1.Descriptor) bool { return e.Digest == d.Digest }) {
			descs = append(descs, gcrv1.Descriptor{MediaType: d.MediaType, Digest: d.Digest, Size: d.Size, ArtifactType: d.ArtifactType})
		}
	}
	if len(descs) == 0 {
		return nil
	}
	b, err := json.Marshal(descs)
	if err != nil {
		return nil
	}
	return map[string][]byte{attest.Referrer.Type: b}
}

// decodeVerifiedReferrers decodes what verifiedReferrersPayload cached. Each
// descriptor needs a digest to fetch by, and must be of artifactType, the type
// ImageData records it under.
func decodeVerifiedReferrers(data []byte, artifactType string) ([]gcrv1.Descriptor, bool) {
	var descs []gcrv1.Descriptor
	if err := json.Unmarshal(data, &descs); err != nil || len(descs) == 0 {
		return nil, false
	}
	for _, d := range descs {
		if d.Digest.Hex == "" || d.ArtifactType != artifactType {
			return nil, false
		}
	}
	return descs, true
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
		img, err := f.imgCtx.Get(ctx, image, WithRequestContext(ctx, f.authOpts), f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		key := pendingKey(image, attestation)

		// Complete a deferred restore from a same-request cache hit: only
		// now, since extractPayload() was actually called, apply the cached
		// payload to this fresh ImageData.
		if payloads, ok := f.pendingIntotoRestores[key]; ok {
			for predicateType, data := range payloads {
				img.AddVerifiedIntotoPayloads(predicateType, data)
			}
			delete(f.pendingIntotoRestores, key)
		}
		if referrers, ok := f.pendingReferrerRestores[key]; ok {
			for _, desc := range referrers {
				img.AddVerifiedReferrer(desc)
			}
			delete(f.pendingReferrerRestores, key)
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
