package imageverify

import (
	"context"
	"encoding/json"
	"fmt"
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
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	signatureCacheRule   = "verifyImageSignatures"
	attestationCacheRule = "verifyAttestationSignatures"
)

type ivfuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext
	policy          v1beta1.ImageValidatingPolicyLike
	creds           *v1beta1.Credentials
	imgRules        []compiler.MatchImageReference
	attestationList map[string]v1beta1.Attestation
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
	ivCache         imageverifycache.Client
	authOpts        []remote.Option
	nameOpts        []name.Option
	verifications   *ImageVerificationResults
}

func ImageVerifyCELFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister corev1listers.SecretLister,
	ivCache imageverifycache.Client,
	adapter types.Adapter,
	verifications *ImageVerificationResults,
) (*ivfuncs, error) {
	if ivpol == nil {
		return nil, fmt.Errorf("nil image verification policy")
	}
	env, err := compiler.NewMatchImageEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL image verification env: %v", err)
	}

	spec := ivpol.GetSpec()
	imgRules, errs := compiler.CompileMatchImageReferences(field.NewPath("spec", "MatchImageReferences"), env, spec.MatchImageReferences...)
	if errs != nil {
		return nil, fmt.Errorf("failed to compile matches: %v", errs.ToAggregate())
	}

	// by default, try to use the options built globally from flags
	authOpts, nameOpts := registryclient.GlobalOptsOrDefault(context.Background())
	if spec.Credentials != nil {
		authOpts, nameOpts = regcreds.RemoteOptsFromIvpolCredentials(lister, *spec.Credentials, config.KyvernoNamespace())
	}

	return &ivfuncs{
		Adapter:         adapter,
		logger:          logger,
		imgCtx:          imgCtx,
		policy:          ivpol,
		creds:           spec.Credentials,
		imgRules:        imgRules,
		attestationList: attestationMap(ivpol),
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
		ivCache:         ivCache,
		nameOpts:        nameOpts,
		authOpts:        authOpts[:],
		verifications:   verifications,
	}, nil
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

func (f *ivfuncs) verify_image_signature_string_stringarray(image ref.Val, attestors ref.Val) ref.Val {
	ctx := context.TODO()
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
		img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for _, attestor := range attestors {
			if attestor.IsCosign() {
				f.logger.V(4).Info("verifying image signature", "image", image, "attestor", attestor.Name, "type", "cosign")
				if err := f.cosignVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
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

func (f *ivfuncs) verify_image_attestations_string_string_stringarray(args ...ref.Val) ref.Val {
	ctx := context.TODO()
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
		cacheRule := attestorCacheRule(attestationCacheRule, attestation, attestors)
		if f.ivCache != nil {
			if found, payloads, err := f.ivCache.GetWithPayload(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				if err := f.restoreCachedIntotoPayloads(ctx, image, attestation, payloads); err != nil {
					// A degraded cache entry (missing payload, or the restore itself
					// failed) can't be trusted as a hit. Fall back to full
					// re-verification below rather than denying an admission that
					// was already verified once.
					f.logger.V(4).Info("cache hit could not be restored, falling back to re-verification", "image", image, "attestation", attestation, "reason", err.Error())
				} else {
					f.logger.V(4).Info("image attestation verification cache hit", "image", image, "policy", f.policy.GetName())
					f.verifications.Record(image, true)
					return f.NativeToValue(len(attestors))
				}
			}
		}
		// Hoist invariant lookups out of the loop: both the attestation
		// definition and the image data are the same for every attestor.
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}
		img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for _, attestor := range attestors {
			if attestor.IsCosign() {
				f.logger.V(4).Info("verifying attestation signature", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "cosign")
				if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
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
					f.logger.V(6).Info("attestation signature verification failed", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary", "error", err)
				} else {
					f.logger.V(4).Info("attestation signature verified", "image", image, "attestation", attestation, "attestor", attestor.Name, "type", "notary")
					count += 1
				}
			}
		}
		f.logger.V(6).Info("verifyAttestationSignatures returning", "image", image, "attestation", attestation, "verifiedCount", count)
		if f.ivCache != nil && len(attestors) > 0 && count == len(attestors) {
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

// restoreCachedIntotoPayloads repopulates verifiedIntotoPayloads on the request's
// ImageData after an attestation verification cache hit, so extractPayload works.
// Returns an error if the attestation needs a cached payload but none is
// available (or restoring it fails), signaling the caller to fall back to full
// re-verification instead of trusting a degraded cache hit.
func (f *ivfuncs) restoreCachedIntotoPayloads(ctx context.Context, image, attestation string, payloads map[string][]byte) error {
	attest, ok := f.attestationList[attestation]
	if !ok || !attest.IsInToto() {
		return nil
	}
	if len(payloads) == 0 {
		return fmt.Errorf("cached verification found but no payload available for attestation %s", attestation)
	}
	img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
	if err != nil {
		return fmt.Errorf("failed to get imagedata: %v", err)
	}
	for predicateType, data := range payloads {
		img.AddVerifiedIntotoPayloads(predicateType, data)
	}
	return nil
}

// intotoPayloadsFromImage reads verified intoto payloads from ImageData after a
// successful Cosign attestation verify. ImageData does not expose a getter for
// the raw map, so we round-trip through GetPayload + json.Marshal.
//
// Safe degrade: if GetPayload (or Marshal) fails after verification already
// succeeded, we return nil and the caller skips caching this result entirely
// rather than recording a presence-only entry. Prefer re-verifying next time
// over a cache hit with no payload to restore.
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

func (f *ivfuncs) payload_string_string(image ref.Val, attestation ref.Val) ref.Val {
	ctx := context.TODO()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else if attestation, err := utils.ConvertToNative[string](attestation); err != nil {
		return types.WrapErr(err)
	} else {
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}
		img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		payload, err := img.GetPayload(attest)
		if err != nil {
			return types.NewErr("failed to get payload: %v", err)
		}
		return f.NativeToValue(payload)
	}
}

func (f *ivfuncs) get_image_data_string(image ref.Val) ref.Val {
	ctx := context.TODO()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else {
		img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		return f.NativeToValue(*img)
	}
}
