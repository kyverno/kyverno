package imageverify

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/image/verifiers/ivpol/cosign"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
)

// fakeImageContext returns an empty ImageData without touching a registry so
// the CEL functions reach the verifier call hermetically.
type fakeImageContext struct{}

func (fakeImageContext) AddImages(ctx context.Context, images []string, authOpts []remote.Option, nameOpts []name.Option) error {
	return nil
}

func (fakeImageContext) Get(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	return &imagedataloader.ImageData{}, nil
}

// fakeCosignVerifier returns a fixed error from both verification methods.
type fakeCosignVerifier struct {
	err error
}

func (f fakeCosignVerifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor *v1beta1.Attestor) error {
	return f.err
}

func (f fakeCosignVerifier) VerifyAttestationSignature(ctx context.Context, image *imagedataloader.ImageData, attestation *v1beta1.Attestation, attestor *v1beta1.Attestor) error {
	return f.err
}

func newSetupErrorFuncs(t *testing.T, verifyErr error) *ivfuncs {
	t.Helper()
	return &ivfuncs{
		Adapter:        types.DefaultTypeAdapter,
		logger:         logr.Discard(),
		imgCtx:         fakeImageContext{},
		policy:         &v1beta1.ImageValidatingPolicy{},
		cosignVerifier: fakeCosignVerifier{err: verifyErr},
		attestationList: map[string]v1beta1.Attestation{
			"vsa": {
				Name:   "vsa",
				InToto: &v1beta1.InToto{Type: "https://slsa.dev/verification_summary/v1"},
			},
		},
		verifications: NewImageVerificationResults(),
	}
}

func cosignAttestors() []v1beta1.Attestor {
	return []v1beta1.Attestor{{Name: "producer", Cosign: &v1beta1.Cosign{}}}
}

// A cosign setup error must surface as a CEL evaluation error (RuleStatusError),
// not a silent verified-count of 0, so failurePolicy applies and the cause is
// diagnosable.
func Test_impl_verify_image_signature_setup_error_surfaces_as_cel_error(t *testing.T) {
	f := newSetupErrorFuncs(t, cosign.Setup(errors.New("failed to build cosign verification opts")))

	out := f.verify_image_signature_string_stringarray(f.NativeToValue("ghcr.io/org/img:tag"), f.NativeToValue(cosignAttestors()))

	assert.True(t, types.IsError(out), "expected a CEL error for a setup failure, got: %v", out.Value())
}

// A genuine (non-setup) verification failure must keep the existing behavior: a
// verified-count of 0, never a CEL error.
func Test_impl_verify_image_signature_non_setup_error_returns_zero(t *testing.T) {
	f := newSetupErrorFuncs(t, errors.New("signature not found"))

	out := f.verify_image_signature_string_stringarray(f.NativeToValue("ghcr.io/org/img:tag"), f.NativeToValue(cosignAttestors()))

	assert.False(t, types.IsError(out), "a genuine verification failure must not be a CEL error: %v", out.Value())
	assert.Equal(t, int64(0), out.Value())
}

func Test_impl_verify_attestation_signature_setup_error_surfaces_as_cel_error(t *testing.T) {
	f := newSetupErrorFuncs(t, cosign.Setup(errors.New("failed to build cosign verification opts")))

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue("ghcr.io/org/img:tag"),
		f.NativeToValue("vsa"),
		f.NativeToValue(cosignAttestors()),
	)

	assert.True(t, types.IsError(out), "expected a CEL error for a setup failure, got: %v", out.Value())
}

func Test_impl_verify_attestation_signature_non_setup_error_returns_zero(t *testing.T) {
	f := newSetupErrorFuncs(t, errors.New("attestation not found"))

	out := f.verify_image_attestations_string_string_stringarray(
		f.NativeToValue("ghcr.io/org/img:tag"),
		f.NativeToValue("vsa"),
		f.NativeToValue(cosignAttestors()),
	)

	assert.False(t, types.IsError(out), "a genuine verification failure must not be a CEL error: %v", out.Value())
	assert.Equal(t, int64(0), out.Value())
}
