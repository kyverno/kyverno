package imageverify

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ivfuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext
	ivpol           v1beta1.ImageValidatingPolicyLike
	creds           *v1beta1.Credentials
	imgRules        []compiler.MatchImageReference
	attestationList map[string]v1beta1.Attestation
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
	cache           imageverifycache.Client
}

func ImageVerifyCELFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister k8scorev1.SecretInterface,
	adapter types.Adapter,
	cache imageverifycache.Client,
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
	if cache == nil {
		cache = imageverifycache.DisabledImageVerifyCache()
	}
	return &ivfuncs{
		Adapter:         adapter,
		imgCtx:          imgCtx,
		ivpol:           ivpol,
		creds:           spec.Credentials,
		imgRules:        imgRules,
		attestationList: attestationMap(ivpol),
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
		cache:           cache,
	}, nil
}

// signatureCacheKey builds a stable cache key for a signature verification
// performed by a given attestor entry. Verification results are cached per
// list entry so duplicate/empty attestor names do not collide.
func signatureCacheKey(attestorIndex int, attestor v1beta1.Attestor) string {
	name := attestor.Name
	if name == "" {
		name = "unnamed"
	}
	return fmt.Sprintf("signature:%d:%s", attestorIndex, name)
}

// attestationCacheKey builds a stable cache key for an attestation
// verification performed by a given attestor entry for a given attestation
// name. The key includes the attestor index to prevent collisions for
// duplicate/empty names.
func attestationCacheKey(attestation string, attestorIndex int, attestor v1beta1.Attestor) string {
	name := attestor.Name
	if name == "" {
		name = "unnamed"
	}
	return fmt.Sprintf("attestation:%s:%d:%s", attestation, attestorIndex, name)
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
			return f.NativeToValue(count)
		}

		// Fetch image data once before the loop: the image reference and
		// credentials are the same for every attestor.
		opts := GetRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(ctx, image, opts...)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for i, attestor := range attestors {
			cacheKey := signatureCacheKey(i, attestor)
			if cached, err := f.cache.Get(ctx, f.ivpol, cacheKey, image, true); err == nil && cached {
				count += 1
				continue
			}

			verified := false
			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
					f.logger.Info("failed to verify image cosign", "error", err)
				} else {
					verified = true
				}
			} else if attestor.IsNotary() {
				var certs, tsaCerts string
				if attestor.Notary.Certs != nil {
					certs = attestor.Notary.Certs.Value
				}
				if attestor.Notary.TSACerts != nil {
					tsaCerts = attestor.Notary.TSACerts.Value
				}
				if err := f.notaryVerifier.VerifyImageSignature(ctx, img, certs, tsaCerts); err != nil {
					f.logger.Info("failed to verify image notary", "error", err)
				} else {
					verified = true
				}
			}
			if verified {
				count += 1
				if _, err := f.cache.Set(ctx, f.ivpol, cacheKey, image, true); err != nil {
					f.logger.V(4).Info("failed to update image signature verification cache", "error", err)
				}
			}
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
			return f.NativeToValue(count)
		}

		// Hoist invariant lookups out of the loop: both the attestation
		// definition and the image data are the same for every attestor.
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}
		opts := GetRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(ctx, image, opts...)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}

		for i, attestor := range attestors {
			cacheKey := attestationCacheKey(attestation, i, attestor)
			if cached, err := f.cache.Get(ctx, f.ivpol, cacheKey, image, true); err == nil && cached {
				count += 1
				continue
			}

			verified := false
			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
					f.logger.Info("failed to verify attestation cosign", "error", err)
				} else {
					verified = true
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
				if err := f.notaryVerifier.VerifyAttestationSignature(ctx, img, attest.Referrer.Type, certs, tsaCerts); err != nil {
					f.logger.Info("failed to verify attestation notary", "error", err)
				} else {
					verified = true
				}
			}
			if verified {
				count += 1
				if _, err := f.cache.Set(ctx, f.ivpol, cacheKey, image, true); err != nil {
					f.logger.V(4).Info("failed to update image attestation verification cache", "error", err)
				}
			}
		}
		return f.NativeToValue(count)
	}
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
		opts := GetRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(ctx, image, opts...)
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
		opts := GetRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(ctx, image, opts...)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		return f.NativeToValue(*img)
	}
}
