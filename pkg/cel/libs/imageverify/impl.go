package imageverify

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/cosign"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
	"github.com/kyverno/kyverno/pkg/imageverification/match"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ivfuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext
	creds           *v1alpha1.Credentials
	imgRules        []*match.CompiledMatch
	attestorList    map[string]v1alpha1.Attestor
	attestationList map[string]v1alpha1.Attestation
	lister          k8scorev1.SecretInterface
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
}

func ImageVerifyCELFuncs(logger logr.Logger, imgCtx imagedataloader.ImageContext, ivpol *v1alpha1.ImageValidatingPolicy, lister k8scorev1.SecretInterface, adapter types.Adapter) (*ivfuncs, error) {
	if ivpol == nil {
		return nil, fmt.Errorf("nil image verification policy")
	}

	imgRules, err := match.CompileMatches(field.NewPath("spec", "imageRules"), ivpol.Spec.ImageRules)
	if err != nil {
		return nil, fmt.Errorf("failed to compile matches: %v", err.ToAggregate())
	}

	return &ivfuncs{
		Adapter:         adapter,
		imgCtx:          imgCtx,
		creds:           ivpol.Spec.Credentials,
		imgRules:        imgRules,
		attestorList:    attestorMap(ivpol),
		attestationList: attestationMap(ivpol),
		lister:          lister,
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
	}, nil
}

func (f *ivfuncs) verify_image_signature_string_stringarray(ctx context.Context) func(ref.Val, ref.Val) ref.Val {
	return func(_image ref.Val, _attestors ref.Val) ref.Val {
		if image, err := utils.ConvertToNative[string](_image); err != nil {
			return types.WrapErr(err)
		} else if attestors, err := utils.ConvertToNative[[]string](_attestors); err != nil {
			return types.WrapErr(err)
		} else {
			count := 0
			if match, err := match.Match(f.imgRules, image); err != nil {
				return types.WrapErr(err)
			} else if !match {
				return f.NativeToValue(count)
			}

			for _, attr := range attestors {
				attestor, ok := f.attestorList[attr]
				if !ok {
					return types.NewErr("attestor not found in policy: %s", attr)
				}

				opts := GetRemoteOptsFromPolicy(f.creds)
				img, err := f.imgCtx.Get(ctx, image, opts...)
				if err != nil {
					return types.NewErr("failed to get imagedata: %v", err)
				}

				if attestor.IsCosign() {
					if err := f.cosignVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
						f.logger.Info("failed to verify image cosign: %v", err)
					} else {
						count += 1
					}
				} else if attestor.IsNotary() {
					if err := f.notaryVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
						f.logger.Info("failed to verify image notary: %v", err)
					} else {
						count += 1
					}
				}
			}
			return f.NativeToValue(count)
		}
	}
}

func (f *ivfuncs) verify_image_attestations_string_string_stringarray(ctx context.Context) func(...ref.Val) ref.Val {
	return func(args ...ref.Val) ref.Val {
		if len(args) != 3 {
			return types.NewErr("function usage: <image> <attestation> <attestor list>")
		}
		if image, err := utils.ConvertToNative[string](args[0]); err != nil {
			return types.WrapErr(err)
		} else if attestation, err := utils.ConvertToNative[string](args[1]); err != nil {
			return types.WrapErr(err)
		} else if attestors, err := utils.ConvertToNative[[]string](args[2]); err != nil {
			return types.WrapErr(err)
		} else {
			count := 0
			if match, err := match.Match(f.imgRules, image); err != nil {
				return types.WrapErr(err)
			} else if !match {
				return f.NativeToValue(count)
			}

			for _, attr := range attestors {
				attestor, ok := f.attestorList[attr]
				if !ok {
					return types.NewErr("attestor not found in policy: %s", attr)
				}

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
					if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
						f.logger.Info("failed to verify attestation cosign: %v", err)
					} else {
						count += 1
					}
				} else if attestor.IsNotary() {
					if err := f.notaryVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
						f.logger.Info("failed to verify attestation notary: %v", err)
					} else {
						count += 1
					}
				}
			}
			return f.NativeToValue(count)
		}
	}
}

func (f *ivfuncs) payload_string_string(ctx context.Context) func(_image ref.Val, _attestation ref.Val) ref.Val {
	return func(_image ref.Val, _attestation ref.Val) ref.Val {
		if image, err := utils.ConvertToNative[string](_image); err != nil {
			return types.WrapErr(err)
		} else if attestation, err := utils.ConvertToNative[string](_attestation); err != nil {
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
}

func (f *ivfuncs) get_image_data_string(ctx context.Context) func(_image ref.Val) ref.Val {
	return func(_image ref.Val) ref.Val {
		if image, err := utils.ConvertToNative[string](_image); err != nil {
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
}
