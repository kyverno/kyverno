package imageverify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type diagnosticVerifier struct {
	err   error
	calls int
}

func (v *diagnosticVerifier) VerifyImageSignature(_ context.Context, _ *imagedataloader.ImageData, a *policiesv1beta1.Attestor) error {
	v.calls++
	if a.Name == "valid" {
		return nil
	}
	return v.err
}
func (v *diagnosticVerifier) VerifyAttestationSignature(ctx context.Context, img *imagedataloader.ImageData, _ *policiesv1beta1.Attestation, a *policiesv1beta1.Attestor) error {
	return v.VerifyImageSignature(ctx, img, a)
}

func TestVerificationDiagnosticsWindows(t *testing.T) {
	t.Parallel()
	d := &verificationDiagnostics{}
	r := Runtime{functions: &IvFuncs{diagnostics: d}}
	require.Empty(t, r.VerificationDiagnostics())
	d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "first", "", errors.New("invalid signature"))
	d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "first", "", errors.New("invalid signature"))
	d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "second", "proof", context.Canceled)
	snapshot := r.VerificationDiagnostics()
	require.Equal(t, `; verification details: image "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", attestor "first": invalid signature; image "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", attestor "second", attestation "proof": context canceled`, snapshot)
	r.BeginValidation()
	require.Empty(t, r.VerificationDiagnostics())
	d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "first", "", errors.New("invalid signature"))
	require.Contains(t, r.VerificationDiagnostics(), "invalid signature")
	require.Contains(t, snapshot, "context canceled", "snapshot must survive window reset")
	var zero Runtime
	zero.BeginValidation()
	require.Empty(t, zero.VerificationDiagnostics())
}

func TestVerificationDiagnosticsBounded(t *testing.T) {
	t.Parallel()
	for _, many := range []bool{false, true} {
		t.Run(fmt.Sprint(many), func(t *testing.T) {
			t.Parallel()
			d := &verificationDiagnostics{}
			if many {
				for i := range 10000 {
					d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", fmt.Sprint(i), "", errors.New("失敗"))
				}
			} else {
				d.record("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "attestor", "", errors.New(strings.Repeat("失敗", 10000)))
			}
			r := Runtime{functions: &IvFuncs{diagnostics: d}}
			text := r.VerificationDiagnostics()
			require.LessOrEqual(t, len(text), diagnosticBudget)
			require.True(t, utf8.ValidString(text))
			require.True(t, strings.HasSuffix(text, diagnosticTruncated))
			require.LessOrEqual(t, d.size, diagnosticBudget)
			r.BeginValidation()
			require.Empty(t, r.VerificationDiagnostics())
		})
	}
}

func TestCosignVerificationDiagnosticsCounts(t *testing.T) {
	t.Parallel()
	for _, attestation := range []bool{false, true} {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("registry: unauthorized"), errors.New("signatures not found"), errors.New("invalid signature"), errors.New("annotation mismatch")} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("attestation=%t/%s/reverse=%t", attestation, cause, reverse), func(t *testing.T) {
					t.Parallel()
					env, err := cel.NewEnv(Lib(), cel.Variable("attestors", cel.ListType(cel.DynType)))
					require.NoError(t, err)
					expression := `verifyImageSignatures("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", attestors)`
					if attestation {
						expression = `verifyAttestationSignatures("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", "proof", attestors)`
					}
					policy := &policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "diagnostics", UID: "diagnostics"}, Spec: policiesv1beta1.ImageValidatingPolicySpec{Attestations: []policiesv1beta1.Attestation{{Name: "proof", InToto: &policiesv1beta1.InToto{Type: "proof"}}}}}
					ivFuncs := mustNewIvFuncs(t, logr.Discard(), policy, nil, env.CELTypeAdapter(), nil)
					results := NewImageVerificationResults()
					cache, err := imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
					require.NoError(t, err)
					r := NewRuntimeForPolicy(context.Background(), ivFuncs, runtimeImages{image: &imagedataloader.ImageData{}}, cache, results)
					verifier := &diagnosticVerifier{err: cause}
					r.functions.cosignVerifier = verifier
					attestors := []policiesv1beta1.Attestor{{Name: "invalid", Cosign: &policiesv1beta1.Cosign{}}, {Name: "valid", Cosign: &policiesv1beta1.Cosign{}}}
					if reverse {
						attestors[0], attestors[1] = attestors[1], attestors[0]
					}
					for _, threshold := range []int{0, 1} {
						ast, issues := env.Compile(fmt.Sprintf("%s > %d", expression, threshold))
						require.NoError(t, issues.Err())
						program, err := env.Program(ast)
						require.NoError(t, err)
						r.BeginValidation()
						out, _, err := program.Eval(map[string]any{RuntimeKey: r, "attestors": attestors})
						require.NoError(t, err)
						require.Equal(t, threshold == 0, out.Value())
						require.Contains(t, r.VerificationDiagnostics(), cause.Error())
						if attestation {
							require.Contains(t, r.VerificationDiagnostics(), `attestation "proof"`)
						}
					}
					require.Equal(t, 4, verifier.calls)
					verified, attempted := results.Status("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
					require.True(t, verified)
					require.True(t, attempted)
					rule := mustCacheRule(t, signatureCacheRule, "", attestors)
					if attestation {
						rule = mustCacheRule(t, attestationCacheRule, "proof", attestors)
					}
					found, err := cache.Get(context.Background(), policy, rule, "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", true)
					require.NoError(t, err)
					require.False(t, found, "partial verification must not be cached")
					require.Nil(t, ivFuncs.diagnostics)
					other := NewRuntimeForPolicy(context.Background(), ivFuncs, nil, nil, nil)
					require.Empty(t, other.VerificationDiagnostics())
				})
			}
		}
	}
}

func TestVerificationDiagnosticsCacheHit(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv(Lib(), cel.Variable("attestors", cel.ListType(cel.DynType)))
	require.NoError(t, err)
	policy := &policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "cached", UID: "cached"}}
	ivFuncs := mustNewIvFuncs(t, logr.Discard(), policy, nil, env.CELTypeAdapter(), nil)
	cache, err := imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
	require.NoError(t, err)
	attestors := []policiesv1beta1.Attestor{{Name: "valid", Cosign: &policiesv1beta1.Cosign{}}}
	stored, err := cache.Set(context.Background(), policy, mustCacheRule(t, signatureCacheRule, "", attestors), "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", true)
	require.NoError(t, err)
	require.True(t, stored)
	r := NewRuntimeForPolicy(context.Background(), ivFuncs, nil, cache, nil)
	r.functions.diagnostics.record("previous", "invalid", "", context.Canceled)
	r.BeginValidation()
	ast, issues := env.Compile(`verifyImageSignatures("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105",attestors) > 0`)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := program.Eval(map[string]any{RuntimeKey: r, "attestors": attestors})
	require.NoError(t, err)
	require.Equal(t, true, out.Value())
	require.Empty(t, r.VerificationDiagnostics())
}

func TestCosignVerificationDiagnosticsFailureUncached(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv(Lib(), cel.Variable("attestors", cel.ListType(cel.DynType)))
	require.NoError(t, err)
	policy := &policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "failed", UID: "failed"}}
	ivFuncs := mustNewIvFuncs(t, logr.Discard(), policy, nil, env.CELTypeAdapter(), nil)
	cache, err := imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
	require.NoError(t, err)
	results := NewImageVerificationResults()
	r := NewRuntimeForPolicy(context.Background(), ivFuncs, runtimeImages{image: &imagedataloader.ImageData{}}, cache, results)
	verifier := &diagnosticVerifier{err: context.Canceled}
	r.functions.cosignVerifier = verifier
	attestors := []policiesv1beta1.Attestor{{Name: "invalid", Cosign: &policiesv1beta1.Cosign{}}}
	ast, issues := env.Compile(`verifyImageSignatures("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105",attestors)`)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	for range 2 {
		r.BeginValidation()
		out, _, err := program.Eval(map[string]any{RuntimeKey: r, "attestors": attestors})
		require.NoError(t, err)
		require.Equal(t, int64(0), out.Value())
		require.Contains(t, r.VerificationDiagnostics(), "context canceled")
	}
	require.Equal(t, 2, verifier.calls, "a failed verification must be retried on the next evaluation")
	verified, attempted := results.Status("ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	require.True(t, attempted)
	require.False(t, verified)
	found, err := cache.Get(context.Background(), policy, mustCacheRule(t, signatureCacheRule, "", attestors), "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105", true)
	require.NoError(t, err)
	require.False(t, found)
}

// mustCacheRule derives a cache rule for a test fixture, failing the test if
// the attestors cannot be encoded.
func mustCacheRule(t *testing.T, fn, qualifier string, attestors []policiesv1beta1.Attestor) string {
	t.Helper()
	rule, err := attestorCacheRule(fn, qualifier, attestors)
	require.NoError(t, err)
	return rule
}
