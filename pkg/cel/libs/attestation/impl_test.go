package attestation

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notaryCert is a self-signed test certificate matching the one used in
// pkg/cel/libs/imageverify/impl_test.go for consistency.
const notaryCert = `-----BEGIN CERTIFICATE-----
MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
-----END CERTIFICATE-----`

// newEnv creates a CEL environment with the attestation lib registered and an
// "attestors" map variable for use in test expressions.
func newEnv(t *testing.T) *cel.Env {
	t.Helper()
	imgCtx, err := imagedataloader.NewImageContext(nil)
	require.NoError(t, err)
	env, err := cel.NewEnv(
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(Latest(), imgCtx, nil),
	)
	require.NoError(t, err)
	return env
}

// notaryAttestors returns a map with a single Notary attestor keyed by name.
func notaryAttestors(name string) map[string]v1beta1.Attestor {
	return map[string]v1beta1.Attestor{
		name: {
			Name: name,
			Notary: &v1beta1.Notary{
				Certs: &v1beta1.StringOrExpression{Value: notaryCert},
			},
		},
	}
}

// --- compilation tests ---

func TestLib_CompileVerifyImageSignatures(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`verifyImageSignatures("ghcr.io/kyverno/test-verify-image:signed", [attestors.notary])`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
}

func TestLib_CompileVerifyAttestationSignatures(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`verifyAttestationSignatures("ghcr.io/kyverno/test-verify-image:signed", "sbom/cyclone-dx", [attestors.notary])`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
}

func TestLib_CompileGetImageData(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`getImageData("ghcr.io/kyverno/test-verify-image:signed")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
}

func TestLib_CompileExtractPayload(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`extractPayload("ghcr.io/kyverno/test-verify-image:signed", "https://slsa.dev/provenance/v1")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
}

// TestLib_NoImageMatchGuard verifies that the attestation lib does not restrict
// the OCI reference to any image-reference pattern (unlike imageverify which
// applies the policy's matchImageReferences filter).
func TestLib_NoImageMatchGuard(t *testing.T) {
	env := newEnv(t)
	// An arbitrary OCI reference that would never match typical image patterns
	// must still compile and evaluate without an "image not matched" early-exit.
	ast, issues := env.Compile(`verifyImageSignatures("oci://example.invalid/custom/artifact:latest", [attestors.notary])`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	require.NoError(t, err)
	// Evaluation will fail at registry fetch (the image doesn't exist), but that
	// is a runtime error — it proves the guard was not applied at compile time and
	// is not an image-match early-return.
	_, _, evalErr := prog.Eval(map[string]any{"attestors": notaryAttestors("notary")})
	assert.Error(t, evalErr, "expected registry fetch error, not silent 0-match early return")
}

// --- evaluation tests ---

func TestVerifyImageSignatures_Notary(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`verifyImageSignatures("ghcr.io/kyverno/test-verify-image:signed", [attestors.notary])`)
	assert.Nil(t, issues)
	prog, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prog.Eval(map[string]any{"attestors": notaryAttestors("notary")})
	require.NoError(t, err)
	assert.Equal(t, int64(1), out.Value())
}

func TestVerifyAttestationSignatures_Notary(t *testing.T) {
	env := newEnv(t)
	ast, issues := env.Compile(`verifyAttestationSignatures("ghcr.io/kyverno/test-verify-image:signed", "sbom/cyclone-dx", [attestors.notary])`)
	assert.Nil(t, issues)
	prog, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prog.Eval(map[string]any{"attestors": notaryAttestors("notary")})
	require.NoError(t, err)
	assert.Equal(t, int64(1), out.Value())
}

// TestLib_NilImgCtx_CreatesInternalContext verifies that passing nil for the
// image context creates a valid (unauthenticated) context internally, so the lib
// registers correctly and produces a compilable environment.
func TestLib_NilImgCtx_CreatesInternalContext(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
		Lib(Latest(), nil, nil), // nil imgCtx and lister
	)
	require.NoError(t, err, "Lib with nil imgCtx/lister must not error during env creation")

	ast, issues := env.Compile(`verifyAttestationSignatures("oci://example.com/artifact:v1", "https://slsa.dev/provenance/v1", [attestors.notary])`)
	assert.Nil(t, issues, "CEL expression must compile even with nil credentials")
	assert.NotNil(t, ast)
}

// TestLib_LibraryNameUnique checks that the library name does not clash with the
// imageverify lib (kyverno.imageverify vs kyverno.attestation).
func TestLib_LibraryNameUnique(t *testing.T) {
	l := &lib{}
	assert.Equal(t, "kyverno.attestation", l.LibraryName())
}

// TestLib_InlineAttestationConstruction verifies the design assumption: a type
// string is sufficient to construct the inline Attestation struct used when
// calling the cosign/notary verifiers, without looking anything up in a policy
// spec.
func TestLib_InlineAttestationConstruction(t *testing.T) {
	const attType = "https://slsa.dev/provenance/v1"
	attest := v1beta1.Attestation{
		InToto: &v1beta1.InToto{Type: attType},
	}
	assert.True(t, attest.IsInToto())
	assert.Equal(t, attType, attest.InToto.Type)
	assert.False(t, attest.IsReferrer())
}
