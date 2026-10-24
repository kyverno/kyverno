package imageverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

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

	// pendingIntotoRestores holds intoto payloads read back from the cache on
	// a verifyAttestationSignatures() hit, keyed by "<image>\x00<attestation>".
	// Applying them to ImageData (which needs an imgCtx.Get()) is deferred
	// until extractPayload() actually asks for that image+attestation, so a
	// policy that only calls verifyAttestationSignatures() never pays for it.
	//
	// The cache *write* on a miss stays eager (see
	// verify_image_attestations_string_string_stringarray): img is already
	// in memory at that point, so caching the payload costs no extra I/O,
	// and deferring it to extractPayload() would mean a verify-only policy
	// never completes the write -- so it would never get a real cache hit
	// again, defeating caching entirely for that common case.
	pendingIntotoRestores map[string]map[string][]byte
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
	newFuncs.requestCtx = cosign.WithCandidateStore(ctx)
	newFuncs.imgCtx = imgCtx
	newFuncs.ivCache = cache
	newFuncs.verifications = results
	newFuncs.authOpts = boundRegistryAuth(ctx, f)
	newFuncs.pendingIntotoRestores = map[string]map[string][]byte{}
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
// so two evaluations never share one, unless they evaluate the same policy under
// one WithSharedRegistryAuth context.
//
// That alone does not isolate policies carrying different spec.Credentials where
// one ImageContext serves several of them (digest mutation, the CLI evaluator):
// its cache is keyed by image reference only, so the first policy to fetch an
// image fixes the options -- keychain as much as puller -- recorded on the
// ImageData that every later policy reuses for it. That sharing predates
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

type sharedRegistryAuthKey struct{}

// sharedRegistryAuth holds the options bound for each policy evaluated under a
// WithSharedRegistryAuth context, keyed by the policy's IvFuncs.
type sharedRegistryAuth struct {
	mu    sync.Mutex
	bound map[*IvFuncs][]remote.Option
}

// WithSharedRegistryAuth returns a context under which every NewRuntimeForPolicy
// for the same policy reuses one puller instead of building its own. It is for a
// caller that evaluates one policy several times for one request -- extraction
// mode evaluates it once per pod template -- so that the request costs that
// policy a single auth handshake per repository. Pullers live as long as the
// returned context, so derive it per request.
func WithSharedRegistryAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, sharedRegistryAuthKey{}, &sharedRegistryAuth{bound: map[*IvFuncs][]remote.Option{}})
}

// boundRegistryAuth returns f's options with a puller bound to them: the one
// already bound for f under a WithSharedRegistryAuth context, or a new one.
// Keying by f keeps policies apart, as each carries its own credentials.
func boundRegistryAuth(ctx context.Context, f *IvFuncs) []remote.Option {
	shared, ok := ctx.Value(sharedRegistryAuthKey{}).(*sharedRegistryAuth)
	if !ok {
		return reuseRegistryAuth(f.authOpts, f.logger)
	}
	shared.mu.Lock()
	defer shared.mu.Unlock()
	opts, ok := shared.bound[f]
	if !ok {
		opts = reuseRegistryAuth(f.authOpts, f.logger)
		shared.bound[f] = opts
	}
	return opts
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

// attestorCacheRule builds a cache key from a CEL function name, a qualifier
// (the attestation name in practice) and the group of attestors.
//
// The attestors are hashed whole rather than by name. An attestor's name is a
// label chosen by the policy author, while its key, certificate and trusted
// root are resolved per request from CEL expressions, so keying on the name
// alone would serve one request's verdict to another that resolved different
// material under the same name. Returns an error if an attestor cannot be
// encoded; the caller must then skip the cache rather than key on a partial
// rule.
func attestorCacheRule(fn string, qualifier string, attestors []v1beta1.Attestor) (string, error) {
	digests := make([]string, 0, len(attestors))
	for _, attestor := range attestors {
		encoded, err := json.Marshal(attestor)
		if err != nil {
			return "", fmt.Errorf("failed to encode attestor %q: %w", attestor.GetKey(), err)
		}
		sum := sha256.Sum256(encoded)
		digests = append(digests, hex.EncodeToString(sum[:]))
	}
	sort.Strings(digests)
	var b strings.Builder
	writeCacheKeyPart(&b, fn)
	writeCacheKeyPart(&b, qualifier)
	for _, digest := range digests {
		writeCacheKeyPart(&b, digest)
	}
	return b.String(), nil
}

func writeCacheKeyPart(b *strings.Builder, part string) {
	fmt.Fprintf(b, "%d:%s|", len(part), part)
}

// pendingKey builds the request-scoped lookup key used by the
// pendingIntotoRestores map.
func pendingKey(image, attestation string) string {
	return image + "\x00" + attestation
}

// attestorsHaveUnbindableKeyMaterial reports whether any attestor verifies with
// key material the cache rule cannot bind. A KMS url names a key that is
// fetched during verification, after the rule is derived, so rotating what sits
// behind the url leaves the rule unchanged and the old verdict live. Inline and
// expression-derived keys are materialized before hashing and so are bound.
func attestorsHaveUnbindableKeyMaterial(attestors []v1beta1.Attestor) bool {
	for _, attestor := range attestors {
		if attestor.Cosign == nil || attestor.Cosign.Key == nil {
			continue
		}
		if attestor.Cosign.Key.KMS != "" {
			return true
		}
	}
	return false
}

func (f *IvFuncs) verify_image_signature_string_stringarray(image ref.Val, attestors ref.Val) ref.Val {
	ctx := f.requestContext()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else if attestors, err := utils.ConvertToNative[[]v1beta1.Attestor](attestors); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		if match, err := matching.MatchImage(ctx, image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			f.logger.V(4).Info("skipping image, no matchImageReferences match", "image", image)
			return f.NativeToValue(count)
		}
		f.logger.V(4).Info("verifyImageSignatures called", "image", image, "attestorCount", len(attestors))

		// create a rule with the given attestors
		cacheRule, cacheRuleErr := attestorCacheRule(signatureCacheRule, "", attestors)
		if cacheRuleErr != nil {
			f.logger.Error(cacheRuleErr, "caching disabled: failed to derive cache rule", "image", image)
		}
		cacheable := cacheRuleErr == nil && !attestorsHaveUnbindableKeyMaterial(attestors)
		if cacheRuleErr == nil && !cacheable {
			f.logger.V(2).Info("caching disabled: attestor verifies with key material the cache rule cannot bind", "image", image)
		}
		if f.ivCache != nil && cacheable {
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
		if f.ivCache != nil && cacheable && len(attestors) > 0 && count == len(attestors) {
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
		cacheRule, cacheRuleErr := attestorCacheRule(attestationCacheRule, attestation, attestors)
		if cacheRuleErr != nil {
			f.logger.Error(cacheRuleErr, "caching disabled: failed to derive cache rule", "image", image)
		}
		cacheable := cacheRuleErr == nil && !attestorsHaveUnbindableKeyMaterial(attestors)
		if cacheRuleErr == nil && !cacheable {
			f.logger.V(2).Info("caching disabled: attestor verifies with key material the cache rule cannot bind", "image", image)
		}
		if f.ivCache != nil && cacheable {
			if found, payloads, err := f.ivCache.GetWithPayload(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				if attest.IsInToto() && len(payloads) == 0 {
					// A degraded entry (cached "found" but no payload to
					// restore) can't be trusted as a hit -- we can't safely
					// defer this decision since we don't know yet whether
					// extractPayload() will be called later in this same
					// evaluation. Fall back to full re-verification below
					// rather than denying an admission that was already
					// verified once.
					f.logger.V(4).Info("cache hit has no payload to restore, falling back to re-verification", "image", image, "attestation", attestation)
				} else {
					if len(payloads) > 0 {
						// Defer applying the payload to ImageData (which
						// needs an imgCtx.Get()) until extractPayload()
						// actually asks for it -- a verify-only policy
						// never pays for it.
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
		if f.ivCache != nil && cacheable && len(attestors) > 0 && count == len(attestors) {
			// The write stays eager: img is already in memory (fetched
			// above for verification), so extracting the payload here costs
			// no extra I/O -- unlike the read-side restore, which does.
			// Deferring this write to extractPayload() would mean a
			// verify-only policy (which never calls it) never completes
			// the write, so it would never get a real cache hit again.
			payloads := intotoPayloadsFromImage(img, attest)
			if attest.IsInToto() && len(payloads) == 0 {
				// Verification succeeded but we couldn't capture the payload to
				// cache alongside it. Skip the cache write entirely rather than
				// recording a presence-only hit: a future admission would see
				// "found" but have nothing to restore, silently reproducing the
				// "cannot be fetch before verifying" error. Leaving this
				// uncached means the next request re-verifies from scratch
				// instead of degrading.
				f.logger.Error(nil, "skipping cache write: failed to capture intoto payload after successful verification", "image", image, "attestation", attestation)
			} else if _, err := f.ivCache.SetWithPayload(ctx, f.policy, cacheRule, image, true, payloads); err != nil {
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
