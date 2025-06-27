package imageverify

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/cosign"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type ivfuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext
	creds           *v1alpha1.Credentials
	imgRules        []compiler.MatchImageReference
	attestationList map[string]v1alpha1.Attestation
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
}

func ImageVerifyCELFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	ivpol *v1alpha1.ImageValidatingPolicy,
	lister k8scorev1.SecretInterface,
	adapter types.Adapter,
) (*ivfuncs, error) {
	if ivpol == nil {
		return nil, fmt.Errorf("nil image verification policy")
	}
	env, err := compiler.NewMatchImageEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL image verification env: %v", err)
	}
	imgRules, errs := compiler.CompileMatchImageReferences(field.NewPath("spec", "MatchImageReferences"), env, ivpol.Spec.MatchImageReferences...)
	if errs != nil {
		return nil, fmt.Errorf("failed to compile matches: %v", errs.ToAggregate())
	}
	return &ivfuncs{
		Adapter:         adapter,
		imgCtx:          imgCtx,
		creds:           ivpol.Spec.Credentials,
		imgRules:        imgRules,
		attestationList: attestationMap(ivpol),
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
	}, nil
}

func (f *ivfuncs) verify_image_signature_string_stringarray(image ref.Val, attestors ref.Val) ref.Val {
	ctx := context.TODO()
	if image, err := utils.ConvertToNative[string](image); err != nil {
		return types.WrapErr(err)
	} else if attestors, err := utils.ConvertToNative[[]v1alpha1.Attestor](attestors); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		if match, err := matching.MatchImage(image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			return f.NativeToValue(count)
		}
		for _, attestor := range attestors {
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
				var certs, tsaCerts string
				if attestor.Notary.Certs != nil {
					certs = attestor.Notary.Certs.Value
				}
				if attestor.Notary.TSACerts != nil {
					tsaCerts = attestor.Notary.TSACerts.Value
				}
				if err := f.notaryVerifier.VerifyImageSignature(ctx, img, certs, tsaCerts); err != nil {
					f.logger.Info("failed to verify image notary: %v", err)
				} else {
					count += 1
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
	} else if attestors, err := utils.ConvertToNative[[]v1alpha1.Attestor](args[2]); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		if match, err := matching.MatchImage(image, f.imgRules...); err != nil {
			return types.WrapErr(err)
		} else if !match {
			return f.NativeToValue(count)
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
				if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
					f.logger.Info("failed to verify attestation cosign: %v", err)
				} else {
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
				if err := f.notaryVerifier.VerifyAttestationSignature(ctx, img, attest.Referrer.Type, certs, tsaCerts); err != nil {
					f.logger.Info("failed to verify attestation notary: %v", err)
				} else {
					count += 1
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
