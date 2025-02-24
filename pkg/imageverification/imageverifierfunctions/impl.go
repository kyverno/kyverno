package imageverifierfunctions

import (
	"context"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/cosign"
	"github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// TODO: remember to use global remote options
type ivfuncs struct {
	types.Adapter

	ctx             context.Context
	imgCtx          imagedataloader.ImageContext
	creds           *v1alpha1.Credentials
	attestorList    map[string]v1alpha1.Attestor
	attestationList map[string]v1alpha1.Attestation
	lister          k8scorev1.SecretInterface
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
}

func ImageVerifyCELFuncs(ctx context.Context, imgCtx imagedataloader.ImageContext, ivpol *v1alpha1.ImageVerificationPolicy, lister k8scorev1.SecretInterface, adapter types.Adapter) *ivfuncs {
	return &ivfuncs{
		Adapter:         adapter,
		ctx:             ctx,
		imgCtx:          imgCtx,
		creds:           ivpol.Spec.Credentials,
		attestorList:    attestorMap(ivpol),
		attestationList: attestationMap(ivpol),
		lister:          lister,
		cosignVerifier:  cosign.NewVerifier(lister),
		notaryVerifier:  notary.NewVerifier(),
	}
}

func (f *ivfuncs) verify_image_signature_string_stringarray(_image ref.Val, _attestors ref.Val) ref.Val {
	if image, err := utils.ConvertToNative[string](_image); err != nil {
		return types.WrapErr(err)
	} else if attestors, err := utils.ConvertToNative[[]string](_attestors); err != nil {
		return types.WrapErr(err)
	} else {
		count := 0
		for _, attr := range attestors {
			attestor, ok := f.attestorList[attr]
			if !ok {
				return types.NewErr("attestor not found in policy: %s", attr)
			}

			opts := getRemoteOptsFromPolicy(f.creds)
			img, err := f.imgCtx.Get(f.ctx, image, opts...)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}

			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyImageSignature(f.ctx, img, &attestor); err != nil {
					return types.NewErr("failed to get imagedata: %v", err)
				} else {
					count += 1
				}
			} else if attestor.IsNotary() {
				if err := f.notaryVerifier.VerifyImageSignature(f.ctx, img, &attestor); err != nil {
					return types.NewErr("failed to get imagedata: %v", err)
				} else {
					count += 1
				}
			}
		}
		return f.NativeToValue(count)
	}
}

func (f *ivfuncs) verify_image_attestations_string_string_stringarray(args ...ref.Val) ref.Val {
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
		for _, attr := range attestors {
			attestor, ok := f.attestorList[attr]
			if !ok {
				return types.NewErr("attestor not found in policy: %s", attr)
			}

			attest, ok := f.attestationList[attestation]
			if !ok {
				return types.NewErr("attestation not found in policy: %s", attestation)
			}

			opts := getRemoteOptsFromPolicy(f.creds)
			img, err := f.imgCtx.Get(f.ctx, image, opts...)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}

			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyAttestationSignature(f.ctx, img, &attest, &attestor); err != nil {
					return types.NewErr("failed to get imagedata: %v", err)
				} else {
					count += 1
				}
			} else if attestor.IsNotary() {
				if err := f.notaryVerifier.VerifyAttestationSignature(f.ctx, img, &attest, &attestor); err != nil {
					return types.NewErr("failed to get imagedata: %v", err)
				} else {
					count += 1
				}
			}
		}
		return f.NativeToValue(count)
	}
}

func (f *ivfuncs) payload_string_string(_image ref.Val, _attestation ref.Val) ref.Val {
	if image, err := utils.ConvertToNative[string](_image); err != nil {
		return types.WrapErr(err)
	} else if attestation, err := utils.ConvertToNative[string](_attestation); err != nil {
		return types.WrapErr(err)
	} else {
		attest, ok := f.attestationList[attestation]
		if !ok {
			return types.NewErr("attestation not found in policy: %s", attestation)
		}

		opts := getRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(f.ctx, image, opts...)
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

func (f *ivfuncs) get_image_data_string(ctx ref.Val, _image ref.Val) ref.Val {
	if image, err := utils.ConvertToNative[string](_image); err != nil {
		return types.WrapErr(err)
	} else {
		opts := getRemoteOptsFromPolicy(f.creds)
		img, err := f.imgCtx.Get(f.ctx, image, opts...)
		if err != nil {
			return types.NewErr("failed to get imagedata: %v", err)
		}
		return f.NativeToValue(*img)
	}
}
