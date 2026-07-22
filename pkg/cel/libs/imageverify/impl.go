package imageverify

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
}

func ImageVerifyCELFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister k8scorev1.SecretInterface,
	ivCache imageverifycache.Client,
	adapter types.Adapter,
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
	}, nil
}

// attestorCacheRule builds a cache rule key that is specific to a function, an optional
// qualifier (e.g. an attestation name), and the exact set of attestors used in a call, so
// that different validations in the same policy version don't collide. Attestor names are
// sorted so the same set produces the same key regardless of call order, and every part is
// length-prefixed so a name containing a delimiter character can't be crafted to collide
// with a different set of parts.
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
		cacheRule := attestorCacheRule(signatureCacheRule, "", attestors)
		if f.ivCache != nil {
			if found, err := f.ivCache.Get(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				f.logger.V(4).Info("image signature verification cache hit", "image", image, "policy", f.policy.GetName())
				return f.NativeToValue(len(attestors))
			}
		}
		for _, attestor := range attestors {
			opts := GetRemoteOptsFromPolicy(f.creds)
			img, err := f.imgCtx.Get(ctx, image, opts...)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}

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
			if found, err := f.ivCache.Get(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache get", "image", image)
			} else if found {
				f.logger.V(4).Info("image attestation verification cache hit", "image", image, "policy", f.policy.GetName())
				return f.NativeToValue(len(attestors))
			}
		}
		for _, attestor := range attestors {
			attest, ok := f.attestationList[attestation]
			if !ok {
				return types.NewErr("attestation not found in policy: %s", attestation)
			}
			opts := GetRemoteOptsFromPolicy(f.creds)
			img, err := f.imgCtx.Get(ctx, image, opts...)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}
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
			if _, err := f.ivCache.Set(ctx, f.policy, cacheRule, image, true); err != nil {
				f.logger.Error(err, "error occurred during image verify cache set", "image", image)
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
