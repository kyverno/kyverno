package attestation

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/notary"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type atfuncs struct {
	types.Adapter

	logger         logr.Logger
	imgCtx         imagedataloader.ImageContext
	cosignVerifier *cosign.Verifier
	notaryVerifier *notary.Verifier
}

func newAtFuncs(
	logger logr.Logger,
	imgCtx imagedataloader.ImageContext,
	lister k8scorev1.SecretInterface,
	adapter types.Adapter,
) (*atfuncs, error) {
	if imgCtx == nil {
		var err error
		imgCtx, err = imagedataloader.NewImageContext(lister)
		if err != nil {
			return nil, err
		}
	}
	return &atfuncs{
		Adapter:        adapter,
		logger:         logger,
		imgCtx:         imgCtx,
		cosignVerifier: cosign.NewVerifier(lister, logger),
		notaryVerifier: notary.NewVerifier(logger),
	}, nil
}

// registerFuncs wires the atfuncs implementations into the CEL environment under
// the same function names used by the imageverify lib. The two libs are never
// registered in the same cel.Env (ivpol compiler vs vpol compiler), so the names
// do not conflict.
func registerFuncs(env *cel.Env, impl *atfuncs) (*cel.Env, error) {
	decls := map[string][]cel.FunctionOpt{
		"verifyImageSignatures": {
			cel.Overload(
				"verify_image_signature_string_stringarray",
				[]*cel.Type{types.StringType, types.NewListType(types.DynType)},
				types.IntType,
				cel.BinaryBinding(impl.verifyImageSignatures),
			),
		},
		"verifyAttestationSignatures": {
			cel.Overload(
				"verify_image_attestations_string_string_stringarray",
				[]*cel.Type{types.StringType, types.StringType, types.NewListType(types.DynType)},
				types.IntType,
				cel.FunctionBinding(impl.verifyAttestationSignatures),
			),
		},
		"getImageData": {
			cel.Overload(
				"get_image_data_string",
				[]*cel.Type{types.StringType},
				types.DynType,
				cel.UnaryBinding(impl.getImageData),
			),
		},
		"extractPayload": {
			cel.Overload(
				"payload_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.DynType,
				cel.BinaryBinding(impl.extractPayload),
			),
		},
	}

	opts := make([]cel.EnvOption, 0, len(decls))
	for name, overloads := range decls {
		opts = append(opts, cel.Function(name, overloads...))
	}
	return env.Extend(opts...)
}

// verifyImageSignatures verifies the image signature for each attestor and returns
// the count of successful matches. Unlike the imageverify lib, no image-reference
// filter is applied — all OCI references are accepted.
func (f *atfuncs) verifyImageSignatures(image ref.Val, attestors ref.Val) ref.Val {
	ctx := context.TODO()
	img, err := utils.ConvertToNative[string](image)
	if err != nil {
		return types.WrapErr(err)
	}
	attestorList, err := utils.ConvertToNative[[]policiesv1beta1.Attestor](attestors)
	if err != nil {
		return types.WrapErr(err)
	}

	count := 0
	imageData, err := f.imgCtx.Get(ctx, img)
	if err != nil {
		return types.NewErr("failed to get image data: %v", err)
	}
	for _, attestor := range attestorList {
		if attestor.IsCosign() {
			if err := f.cosignVerifier.VerifyImageSignature(ctx, imageData, &attestor); err != nil {
				f.logger.Info("failed to verify image cosign signature", "error", err)
			} else {
				count++
			}
		} else if attestor.IsNotary() {
			var certs, tsaCerts string
			if attestor.Notary.Certs != nil {
				certs = attestor.Notary.Certs.Value
			}
			if attestor.Notary.TSACerts != nil {
				tsaCerts = attestor.Notary.TSACerts.Value
			}
			if err := f.notaryVerifier.VerifyImageSignature(ctx, imageData, certs, tsaCerts); err != nil {
				f.logger.Info("failed to verify image notary signature", "error", err)
			} else {
				count++
			}
		}
	}
	return f.NativeToValue(count)
}

// verifyAttestationSignatures verifies that an attestation of the given type is
// present and signed by each attestor. The attestationType argument is used
// directly:
//   - for cosign attestors it is the in-toto predicate type
//     (e.g. "https://slsa.dev/provenance/v1")
//   - for notary attestors it is the OCI referrer artifact type
//     (e.g. "application/vnd.example.sbom.v1")
//
// Returns the count of attestors whose signature verified successfully.
func (f *atfuncs) verifyAttestationSignatures(args ...ref.Val) ref.Val {
	ctx := context.TODO()
	if len(args) != 3 {
		return types.NewErr("function usage: verifyAttestationSignatures(<image>, <attestationType>, <attestors>)")
	}
	image, err := utils.ConvertToNative[string](args[0])
	if err != nil {
		return types.WrapErr(err)
	}
	attestationType, err := utils.ConvertToNative[string](args[1])
	if err != nil {
		return types.WrapErr(err)
	}
	attestorList, err := utils.ConvertToNative[[]policiesv1beta1.Attestor](args[2])
	if err != nil {
		return types.WrapErr(err)
	}

	imageData, err := f.imgCtx.Get(ctx, image)
	if err != nil {
		return types.NewErr("failed to get image data: %v", err)
	}

	count := 0
	for _, attestor := range attestorList {
		if attestor.IsCosign() {
			attest := policiesv1beta1.Attestation{
				InToto: &policiesv1beta1.InToto{Type: attestationType},
			}
			if err := f.cosignVerifier.VerifyAttestationSignature(ctx, imageData, &attest, &attestor); err != nil {
				f.logger.Info("failed to verify attestation cosign signature", "error", err)
			} else {
				count++
			}
		} else if attestor.IsNotary() {
			var certs, tsaCerts string
			if attestor.Notary.Certs != nil {
				certs = attestor.Notary.Certs.Value
			}
			if attestor.Notary.TSACerts != nil {
				tsaCerts = attestor.Notary.TSACerts.Value
			}
			if err := f.notaryVerifier.VerifyAttestationSignature(ctx, imageData, attestationType, certs, tsaCerts); err != nil {
				f.logger.Info("failed to verify attestation notary signature", "error", err)
			} else {
				count++
			}
		}
	}
	return f.NativeToValue(count)
}

// getImageData fetches and returns the raw image data for the given OCI reference.
func (f *atfuncs) getImageData(image ref.Val) ref.Val {
	ctx := context.TODO()
	img, err := utils.ConvertToNative[string](image)
	if err != nil {
		return types.WrapErr(err)
	}
	imageData, err := f.imgCtx.Get(ctx, img)
	if err != nil {
		return types.NewErr("failed to get image data: %v", err)
	}
	return f.NativeToValue(*imageData)
}

// extractPayload fetches and returns the decoded attestation payload for the
// given OCI reference and in-toto predicate type. The attestation must have been
// verified first via verifyAttestationSignatures; otherwise the payload will not
// be available in the image data cache.
func (f *atfuncs) extractPayload(image ref.Val, attestationType ref.Val) ref.Val {
	ctx := context.TODO()
	img, err := utils.ConvertToNative[string](image)
	if err != nil {
		return types.WrapErr(err)
	}
	attType, err := utils.ConvertToNative[string](attestationType)
	if err != nil {
		return types.WrapErr(err)
	}
	imageData, err := f.imgCtx.Get(ctx, img)
	if err != nil {
		return types.NewErr("failed to get image data: %v", err)
	}
	attest := policiesv1beta1.Attestation{
		InToto: &policiesv1beta1.InToto{Type: attType},
	}
	payload, err := imageData.GetPayload(attest)
	if err != nil {
		return types.NewErr("failed to get attestation payload: %v", err)
	}
	return f.NativeToValue(payload)
}
