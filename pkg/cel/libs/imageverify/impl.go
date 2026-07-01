package imageverify

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/kyverno/sdk/extensions/regcreds"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type ivfuncs struct {
	types.Adapter

	logger          logr.Logger
	imgCtx          imagedataloader.ImageContext // the image data getter
	creds           *v1beta1.Credentials         // registry credentials
	imgRules        []compiler.MatchImageReference
	attestationList map[string]v1beta1.Attestation
	cosignVerifier  *cosign.Verifier
	notaryVerifier  *notary.Verifier
	secretLister    corev1listers.SecretLister
	authOpts        []remote.Option
	nameOpts        []name.Option
}

// where does the result of this call get stored ?
func ImageVerifyCELFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	ivpol v1beta1.ImageValidatingPolicyLike,
	lister corev1listers.SecretLister,
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
	authOpts, nameOpts := regcreds.RemoteOptsFromIvpolCredentials(lister, *spec.Credentials, config.KyvernoNamespace())

	return &ivfuncs{
		Adapter:         adapter,
		imgCtx:          imgCtx,
		creds:           spec.Credentials,
		imgRules:        imgRules,
		attestationList: attestationMap(ivpol),
		cosignVerifier:  cosign.NewVerifier(lister, logger),
		notaryVerifier:  notary.NewVerifier(logger),
		nameOpts:        nameOpts,
		authOpts:        authOpts[:],
	}, nil
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

		for _, attestor := range attestors {
			img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}

			// the only two attestor types are cosign and notary
			// obviously
			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyImageSignature(ctx, img, &attestor); err != nil {
					f.logger.Info("failed to verify image cosign", "error", err)
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
					f.logger.Info("failed to verify image notary", "error", err)
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
	} else if attestors, err := utils.ConvertToNative[[]v1beta1.Attestor](args[2]); err != nil {
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
			img, err := f.imgCtx.Get(ctx, image, f.authOpts, f.nameOpts)
			if err != nil {
				return types.NewErr("failed to get imagedata: %v", err)
			}
			if attestor.IsCosign() {
				if err := f.cosignVerifier.VerifyAttestationSignature(ctx, img, &attest, &attestor); err != nil {
					f.logger.Info("failed to verify attestation cosign", "error", err)
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
					f.logger.Info("failed to verify attestation notary", "error", err)
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
