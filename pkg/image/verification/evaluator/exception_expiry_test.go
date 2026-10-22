package evaluator

import (
	"context"
	"testing"
	"time"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCachedPolicyExceptionExpiry(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-expiry"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: func() *bool { b := false; return &b }()},
			MatchImageReferences:     []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "bar", Expression: "object.foo.bar"}},
			Validations:              []admissionregistrationv1.Validation{{Expression: "false", Message: "deny"}},
		},
	}
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	future := metav1.NewTime(time.Now().Add(time.Hour))
	for _, tt := range []struct {
		name   string
		expiry *metav1.Time
		exempt bool
	}{
		{"expired", &past, false}, {"future", &future, true}, {"permanent", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ex := &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: tt.expiry, PolicyRefs: []policiesv1beta1.PolicyRef{{Name: policy.Name, Kind: "ImageValidatingPolicy"}}}}
			result, err := Evaluate(context.Background(), []*CompiledImageValidatingPolicy{{Policy: policy, Exceptions: []*policiesv1beta1.PolicyException{ex}}}, multiImageObj(unsignedImage), nil, nil, nil)
			if err != nil || result[policy.Name] == nil {
				t.Fatalf("evaluate result=%v error=%v", result, err)
			}
			r := result[policy.Name]
			if got := len(r.Exceptions) > 0; got != tt.exempt {
				t.Fatalf("exempt=%v, want %v", got, tt.exempt)
			}
			if !tt.exempt && (r.Result || r.Message != "deny") {
				t.Fatalf("expired exception bypassed validation: %#v", r)
			}
		})
	}
}

// TestMutateDigestCachedPolicyExceptionExpiry verifies that the digest-pinning
// path no longer honors a full exemption once the cached exception expires.
// It uses an in-memory image resolver, not a registry/network request.
func TestMutateDigestCachedPolicyExceptionExpiry(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-mutate-expiry"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "example.com/*"}},
		},
	}
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := metav1.NewTime(now.Add(time.Minute))
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "expiring-digest-exemption"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt: &expiresAt,
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policy.Name,
				Kind: "ImageValidatingPolicy",
			}},
		},
	}
	compiled, errs := NewCompiler(nil).Compile(policy, []*policiesv1beta1.PolicyException{exception})
	require.Empty(t, errs)
	compiled.(*compiledPolicy).exceptions[0].Now = func() time.Time { return now }
	request, attr, pod := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
	require.NoError(t, unstructured.SetNestedSlice(
		pod.Object, []any{map[string]any{
			"name":  "main",
			"image": "example.com/image:tag",
		}}, "spec", "containers",
	))
	mutate := func() int {
		t.Helper()
		patches, err := compiled.MutateDigest(
			context.Background(), mutationImages{}, nil, nil,
			attr, request, nil, pod, nil,
			config.NewDefaultConfiguration(false), nil,
		)
		require.NoError(t, err)
		return len(patches)
	}
	require.Equal(t, 0, mutate(), "active full exception should skip digest mutation")

	// Cached compiled policy is deliberately reused without recompilation.
	now = now.Add(2 * time.Minute)
	require.Equal(t, 1, mutate(), "expired exception must not suppress digest mutation")
}

// Reuse the same compiled image-validating policy and unchanged exception,
// advancing only the evaluation clock across the expiry boundary.
func TestCachedImageValidationExceptionExpiryAcrossEvaluations(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	deadline := metav1.NewTime(now.Add(time.Minute))
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ivpol-cached-expiry"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: func() *bool { b := false; return &b }()},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors: []policiesv1beta1.ImageExtractor{{Name: "bar", Expression: "object.foo.bar"}},
			Validations: []admissionregistrationv1.Validation{{Expression: "false", Message: "deny"}},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt: &deadline,
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: policy.Name, Kind: "ImageValidatingPolicy"}},
		},
	}
	compiled, errs := NewCompiler(nil).Compile(policy, []*policiesv1beta1.PolicyException{exception})
	require.Empty(t, errs)
	compiled.(*compiledPolicy).exceptions[0].Now = func() time.Time { return now }
	evaluate := func() *EvaluationResult {
		t.Helper()
		result, err := compiled.Evaluate(context.Background(), nil, nil, nil, nil, multiImageObj(unsignedImage), nil, false, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		return result
	}
	before := evaluate()
	require.Len(t, before.Exceptions, 1, "active exception must be honored")
	now = now.Add(2 * time.Minute)
	after := evaluate()
	require.Empty(t, after.Exceptions, "expired exception must not be retained")
	require.False(t, after.Result, "expired exception must not bypass validation")
	require.Equal(t, "deny", after.Message)
}
