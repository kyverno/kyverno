package evaluator

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func TestValidate_MatchConstraints(t *testing.T) {
	tests := []struct {
		name      string
		ivpol     *policiesv1beta1.ImageValidatingPolicy
		wantErr   bool
		wantField string
		wantMsg   string
	}{
		{
			name: "nil matchConstraints is rejected",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "nil-constraints"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr:   true,
			wantField: "spec.matchConstraints",
			wantMsg:   "a matchConstraints with at least one resource rule is required",
		},
		{
			name: "empty resourceRules is rejected",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-rules"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
					},
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr:   true,
			wantField: "spec.matchConstraints",
			wantMsg:   "a matchConstraints with at least one resource rule is required",
		},
		{
			name: "valid matchConstraints is accepted",
			ivpol: &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-constraints"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"},
										Resources:   []string{"pods"},
									},
								},
							},
						},
					},
					MatchImageReferences: []policiesv1beta1.MatchImageReference{
						{Glob: "ghcr.io/*"},
					},
					Validations: []admissionregistrationv1.Validation{
						{Expression: "true"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.ivpol, nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantField)
				assert.Contains(t, err.Error(), tt.wantMsg)
			} else {
				// The matchConstraints check should not produce an error.
				// Other compile errors may still be present, so we only
				// verify the matchConstraints error is absent.
				if err != nil {
					assert.NotContains(t, err.Error(), "matchConstraints")
				}
			}
		})
	}
}

func gctxPolicy(namespace string) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gctx", Namespace: namespace},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{Expression: `globalContext.get("cluster-entry", "") == null`},
			},
		},
	}
}

func Test_Validate_NamespacedPolicyRejectsGlobalContext(t *testing.T) {
	_, err := Validate(gctxPolicy("tenant-ns"), nil)
	assert.ErrorContains(t, err, "globalContext.* is not allowed in namespaced policies")
}

func Test_Evaluate_NamespacedPolicyGlobalContextDeniedAtRuntime(t *testing.T) {
	libctx := libs.NewFakeContextProvider()
	libctx.AddGlobalReference("cluster-entry", map[string]any{"leaked": "data"})
	ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
	require.NoError(t, err)
	attr := admission.NewAttributesRecord(nil, nil, schema.GroupVersionKind{}, "tenant-ns", "p", schema.GroupVersionResource{}, "", admission.Create, nil, false, nil)

	tests := []struct {
		name      string
		namespace string
		wantErr   string
	}{
		{
			name:      "namespaced policy is denied",
			namespace: "tenant-ns",
			wantErr:   "not allowed in namespaced policies",
		},
		{
			name:      "cluster-scoped policy reads the entry",
			namespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, errs := NewCompiler(ictx, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(gctxPolicy(tt.namespace), nil, nil)
			require.Empty(t, errs)

			result, err := compiled.Evaluate(context.Background(), ictx, attr, &admissionv1.AdmissionRequest{}, nil, true, libctx)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.False(t, result.Result)
		})
	}
}

func Test_Validate_NamespacedPolicyRejectsGlobalContextInAttestors(t *testing.T) {
	basePolicy := func(attestor policiesv1beta1.Attestor) *policiesv1beta1.ImageValidatingPolicy {
		return &policiesv1beta1.ImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "gctx-attestor", Namespace: "tenant-ns"},
			Spec: policiesv1beta1.ImageValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
				},
				Attestors: []policiesv1beta1.Attestor{attestor},
				Validations: []admissionregistrationv1.Validation{
					{Expression: `images.containers.map(image, verifyImageSignatures(image, [attestors.att])).all(e, e > 0)`},
				},
			},
		}
	}
	gctxExpr := &policiesv1beta1.StringOrExpression{Expression: `globalContext.get("cluster-entry", "")`}

	tests := []struct {
		name     string
		attestor policiesv1beta1.Attestor
	}{
		{
			name:     "cosign key expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Cosign: &policiesv1beta1.Cosign{Key: &policiesv1beta1.Key{Expression: `globalContext.get("cluster-entry", "")`}}},
		},
		{
			name:     "cosign certificate expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Cosign: &policiesv1beta1.Cosign{Certificate: &policiesv1beta1.Certificate{Certificate: gctxExpr}}},
		},
		{
			name:     "cosign certificate chain expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Cosign: &policiesv1beta1.Cosign{Certificate: &policiesv1beta1.Certificate{CertificateChain: gctxExpr}}},
		},
		{
			name:     "cosign trusted root expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Cosign: &policiesv1beta1.Cosign{TrustedRoot: gctxExpr}},
		},
		{
			name:     "notary certs expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Notary: &policiesv1beta1.Notary{Certs: gctxExpr}},
		},
		{
			name:     "notary tsa certs expression",
			attestor: policiesv1beta1.Attestor{Name: "att", Notary: &policiesv1beta1.Notary{TSACerts: gctxExpr}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(basePolicy(tt.attestor), nil)
			assert.ErrorContains(t, err, "globalContext.* is not allowed in namespaced policies")
		})
	}

	t.Run("cluster-scoped policy may use globalContext in attestors", func(t *testing.T) {
		pol := basePolicy(tests[0].attestor)
		pol.Namespace = ""
		_, err := Validate(pol, nil)
		assert.NoError(t, err)
	})
}
