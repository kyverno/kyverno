package admissionpolicy

import (
	"fmt"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func exception(namespace, name string, validations ...string) policiesv1beta1.PolicyException {
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "match",
				Expression: "true",
			}},
		},
	}
	for _, expression := range validations {
		polex.Spec.Validations = append(polex.Spec.Validations, admissionregistrationv1.Validation{
			Expression: expression,
		})
	}
	return polex
}

func Test_CanGenerateNativePolicy(t *testing.T) {
	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{{
		name:       "no exceptions",
		exceptions: nil,
		want:       true,
	}, {
		name:       "exception without compensating controls",
		exceptions: []policiesv1beta1.PolicyException{exception("default", "polex-1")},
		want:       true,
	}, {
		name:       "exception with compensating controls",
		exceptions: []policiesv1beta1.PolicyException{exception("default", "polex-1", "object.metadata.name != ''")},
		want:       false,
		wantReason: "policy exception(s) default/polex-1 define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
	}, {
		name: "only the offending exceptions are named",
		exceptions: []policiesv1beta1.PolicyException{
			exception("default", "plain"),
			exception("default", "guarded", "true"),
			exception("other", "also-guarded", "true"),
		},
		want:       false,
		wantReason: "policy exception(s) default/guarded, other/also-guarded define compensating controls (spec.validations), which a generated ValidatingAdmissionPolicy cannot evaluate",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativePolicy_PartialExceptionConditions(t *testing.T) {
	imageScoped := exception("default", "image-scoped")
	imageScoped.Spec.Images = []string{"ghcr.io/example/app:v1"}

	valueScoped := exception("default", "value-scoped")
	valueScoped.Spec.AllowedValues = []string{"allowed"}

	unconditionalImageScope := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "unconditional-image-scope"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/app:v1"},
		},
	}

	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{
		{
			name:       "image-scoped exception with match conditions",
			exceptions: []policiesv1beta1.PolicyException{imageScoped},
			want:       false,
			wantReason: "policy exception(s) default/image-scoped combine matchConditions with image/value scopes, which a generated ValidatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "value-scoped exception with match conditions",
			exceptions: []policiesv1beta1.PolicyException{valueScoped},
			want:       false,
			wantReason: "policy exception(s) default/value-scoped combine matchConditions with image/value scopes, which a generated ValidatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "image scope without match conditions is still supported",
			exceptions: []policiesv1beta1.PolicyException{unconditionalImageScope},
			want:       true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativeMutatingPolicy(t *testing.T) {
	fullException := exception("default", "full")
	imageScoped := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "image-scoped"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/app:v1"},
		},
	}
	valueScoped := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "value-scoped"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			AllowedValues: []string{"allowed"},
		},
	}
	guarded := exception("default", "guarded", "object.metadata.name != ''")

	testCases := []struct {
		name       string
		exceptions []policiesv1beta1.PolicyException
		want       bool
		wantReason string
	}{
		{
			name:       "full exception with match conditions is representable",
			exceptions: []policiesv1beta1.PolicyException{fullException},
			want:       true,
		},
		{
			name:       "image scope is not natively represented",
			exceptions: []policiesv1beta1.PolicyException{imageScoped},
			want:       false,
			wantReason: "policy exception(s) default/image-scoped use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "value scope is not natively represented",
			exceptions: []policiesv1beta1.PolicyException{valueScoped},
			want:       false,
			wantReason: "policy exception(s) default/value-scoped use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
		{
			name:       "compensating controls are not natively represented",
			exceptions: []policiesv1beta1.PolicyException{guarded},
			want:       false,
			wantReason: "policy exception(s) default/guarded use image/value scopes or compensating controls, which a generated MutatingAdmissionPolicy cannot evaluate",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativeMutatingPolicy(tc.exceptions)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func Test_CanGenerateNativePolicies_PassReportResults(t *testing.T) {
	for _, tt := range []struct {
		name string
		reportResult string
		want bool
	}{
		{name: "default skip", reportResult: "", want: true},
		{name: "explicit skip", reportResult: "skip", want: true},
		{name: "explicit pass", reportResult: "pass", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ex := policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "reporting-exception"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					ReportResult: tt.reportResult,
					MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "test", Expression: "true"}},
				},
			}
			for _, check := range []struct {
				name string
				fn func([]policiesv1beta1.PolicyException) (bool, string)
			}{
				{name: "validating", fn: CanGenerateNativePolicy},
				{name: "mutating", fn: CanGenerateNativeMutatingPolicy},
			} {
				t.Run(check.name, func(t *testing.T) {
					ok, reason := check.fn([]policiesv1beta1.PolicyException{ex})
					assert.Equal(t, tt.want, ok)
					if !tt.want {
						assert.Assert(t, strings.Contains(reason, "spec.reportResult=pass"))
						assert.Assert(t, strings.Contains(reason, "default/reporting-exception"))
					}
				})
			}
		})
	}
}

func Test_CanGenerateNativePolicies_MatchConditionLimit(t *testing.T) {
	globalExceptions := []policiesv1beta1.PolicyException{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "full-one"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "full-two"}},
	}
	conditionedExceptions := make([]policiesv1beta1.PolicyException, 65)
	for i := range conditionedExceptions {
		conditionedExceptions[i] = policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: fmt.Sprintf("conditioned-%d", i)},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{
					Name:       fmt.Sprintf("condition-%d", i),
					Expression: "true",
				}},
			},
		}
	}

	tests := []struct {
		name          string
		exceptions    []policiesv1beta1.PolicyException
		existingCount int
		want          bool
		wantReason    string
	}{
		{
			name:          "multiple global exceptions collapse to one match condition",
			exceptions:    globalExceptions,
			existingCount: 63,
			want:          true,
		},
		{
			name:          "policy and exception conditions cannot exceed limit",
			exceptions:    globalExceptions[:1],
			existingCount: 64,
			want:          false,
			wantReason:    "generated ValidatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64",
		},
		{
			name:          "too many exception conditions skip native generation",
			exceptions:    conditionedExceptions,
			existingCount: 0,
			want:          false,
			wantReason:    "generated ValidatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativePolicyWithMatchConditions(tc.exceptions, tc.existingCount)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantReason, reason)
		})
	}

	got, reason := CanGenerateNativeMutatingPolicyWithMatchConditions(globalExceptions, 63)
	assert.Equal(t, true, got)
	assert.Equal(t, "", reason)
	got, reason = CanGenerateNativeMutatingPolicyWithMatchConditions(globalExceptions[:1], 64)
	assert.Equal(t, false, got)
	assert.Equal(t, "generated MutatingAdmissionPolicy would contain 65 matchConditions; Kubernetes allows at most 64", reason)
}

func TestCanGenerateNativeMutatingPolicyExpressions(t *testing.T) {
	tests := []struct {
		name   string
		policy *policiesv1beta1.MutatingPolicy
		want   bool
	}{
		{
			name:   "no references",
			policy: &policiesv1beta1.MutatingPolicy{},
			want:   true,
		},
		{
			name: "match condition references exception values",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "scope", Expression: "exceptions.allowedImages.size() > 0"}},
			}},
		},
		{
			name: "variable references exception values",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{Name: "values", Expression: "exceptions.allowedValues"}},
			}},
		},
		{
			name: "apply configuration references exception values",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Mutations: []admissionregistrationv1alpha1.Mutation{{ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: "exceptions.allowedImages[0]",
				}}},
			}},
		},
		{
			name: "json patch references exception values",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Mutations: []admissionregistrationv1alpha1.Mutation{{JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: "exceptions.allowedValues[0]",
				}}},
			}},
		},
		{
			name: "target match condition referencing exception values is not supported",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConditions: []admissionregistrationv1.MatchCondition{{Name: "target-scope", Expression: "exceptions.allowedValues.size() > 0"}},
			}},
		},
		{
			name: "target match condition without exception references is not supported",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConditions: []admissionregistrationv1.MatchCondition{{Name: "target", Expression: "object.metadata.name == 'example'"}},
			}},
		},
		{
			name: "target resolver referencing exception values is not supported",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{Expression: "exceptions.allowedImages[0]"},
			}},
		},
		{
			name: "target constraints without an expression are not silently dropped",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{},
			}},
		},
		{
			name: "quoted string is not a variable reference",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{Name: "literal", Expression: "'exceptions.allowedImages'"}},
			}},
			want: true,
		},
		{
			name: "similarly named object field is not a variable reference",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{Name: "resourceField", Expression: "object.exceptions.allowedValues"}},
			}},
			want: true,
		},
		{
			name: "locally shadowed exceptions identifier is not a global reference",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{Name: "local", Expression: "object.items.all(exceptions, exceptions.allowedImages.size() > 0)"}},
			}},
			want: true,
		},
		{
			name: "unparseable exception expression fails closed",
			policy: &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{Name: "broken", Expression: "exceptions.allowedImages["}},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := CanGenerateNativeMutatingPolicyExpressions(tc.policy)
			assert.Equal(t, got, tc.want)
			if tc.want {
				assert.Equal(t, reason, "")
			} else {
				assert.Assert(t, reason != "")
			}
		})
	}
}

func TestCanGenerateNativeMutatingPolicyExpressions_MatchConditionVariables(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       bool
	}{
		{name: "global variable selection", expression: "variables.allowed", want: false},
		{name: "global variable identifier", expression: "variables == null", want: false},
		{name: "absolute global variable", expression: ".variables.allowed", want: false},
		{name: "nested global use", expression: "variables.allowed.exists(x, x)", want: false},
		{name: "ordinary match condition", expression: "object.metadata.name == 'example'", want: true},
		{name: "quoted identifier", expression: "'variables.allowed' == 'variables.allowed'", want: true},
		{name: "similarly named resource field", expression: "object.variables.allowed == 'example'", want: true},
		{name: "local comprehension variable", expression: "object.items.all(variables, variables != null)", want: true},
		{name: "local binding", expression: "cel.bind(variables, object, variables.metadata.name)", want: true},
		{name: "global range with local binding", expression: "variables.allowed.all(variables, variables != null)", want: false},
		{name: "invalid CEL rejected", expression: "variables.allowed[", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "test", Expression: tt.expression}},
			}}
			got, reason := CanGenerateNativeMutatingPolicyExpressions(policy)
			assert.Equal(t, tt.want, got)
			if !tt.want {
				assert.Assert(t, reason != "")
			}
		})
	}
}

func TestCanGenerateNativeMutatingPolicyExpressions_UnsupportedNativeFields(t *testing.T) {
	selectEverything := &metav1.LabelSelector{MatchLabels: map[string]string{"team": "platform"}}
	equivalent := admissionregistrationv1.Equivalent
	tests := []struct {
		name string
		spec policiesv1beta1.MutatingPolicySpec
		want bool
	}{
		{name: "resource rules remain supported", spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{}},
			},
		}, want: true},
		{name: "namespace selector changes scope", spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{NamespaceSelector: selectEverything},
		}},
		{name: "object selector changes scope", spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{ObjectSelector: selectEverything},
		}},
		{name: "explicit match policy is not copied", spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{MatchPolicy: &equivalent},
		}},
		{name: "excluded resources must not be dropped", spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{}},
			},
		}},
		{name: "audit annotations are not copied", spec: policiesv1beta1.MutatingPolicySpec{
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{Key: "audit.kyverno.io/test", ValueExpression: "'x'"}},
		}},
		{name: "bare exceptions reference", spec: policiesv1beta1.MutatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{Name: "x", Expression: "exceptions == null"}},
		}},
		{name: "unknown global exceptions field", spec: policiesv1beta1.MutatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{Name: "x", Expression: "exceptions.otherField"}},
		}},
		{name: "local exceptions binding stays supported", spec: policiesv1beta1.MutatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{Name: "x", Expression: "object.items.all(exceptions, exceptions != null)"}},
		}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := CanGenerateNativeMutatingPolicyExpressions(&policiesv1beta1.MutatingPolicy{Spec: tt.spec})
			assert.Equal(t, tt.want, ok)
			if !tt.want {
				assert.Assert(t, reason != "")
			}
		})
	}
}

func TestCanGenerateNativeValidatingPolicyExpressions(t *testing.T) {
	makeVars := func(n int) []admissionregistrationv1.Variable {
		vars := make([]admissionregistrationv1.Variable, n)
		for i := range vars {
			vars[i] = admissionregistrationv1.Variable{Name: fmt.Sprintf("v%d", i), Expression: "true"}
		}
		return vars
	}
	tests := []struct {
		name       string
		spec       policiesv1beta1.ValidatingPolicySpec
		exceptions []policiesv1beta1.PolicyException
		want       bool
	}{
		{name: "standard policy", want: true},
		{name: "match condition cannot reference native variables", spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "bad", Expression: "variables.test == true"}},
		}},
		{name: "match condition bare variables reference rejected", spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "bad", Expression: "variables == null"}},
		}},
		{name: "match condition string literal remains safe", spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "ok", Expression: "'variables.test' == 'variables.test'"}},
		}, want: true},
		{name: "local variables binding captures generated image variable", spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "object.items.all(variables, exceptions.allowedImages.size() > 0)"}},
		}},
		{name: "local variables binding captures generated value variable", spec: policiesv1beta1.ValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{Name: "x", Expression: "cel.bind(variables, object, exceptions.allowedValues.size())"}},
		}},
		{name: "exact variable cap", spec: policiesv1beta1.ValidatingPolicySpec{
			Variables: makeVars(63),
		}, exceptions: []policiesv1beta1.PolicyException{{Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"example.io/test:v1"},
		}}}, want: true},
		{name: "generated variable count exceeds limit", spec: policiesv1beta1.ValidatingPolicySpec{
			Variables: makeVars(63),
		}, exceptions: []policiesv1beta1.PolicyException{{Spec: policiesv1beta1.PolicyExceptionSpec{
			Images:        []string{"example.io/test:v1"},
			AllowedValues: []string{"hello"},
		}}}},
		{name: "valid images and values under cap", spec: policiesv1beta1.ValidatingPolicySpec{
			Variables: makeVars(62),
		}, exceptions: []policiesv1beta1.PolicyException{{Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"example.io/test:v1"}, AllowedValues: []string{"ok"},
		}}}, want: true},
		{name: "variable reference creates generated variable", spec: policiesv1beta1.ValidatingPolicySpec{
			Variables:   makeVars(64),
			Validations: []admissionregistrationv1.Validation{{Expression: "exceptions.allowedImages.size() > 0"}},
		}},
		{name: "bare exceptions fails closed", spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "exceptions == null"}},
		}},
		{name: "unsupported field fails closed", spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "bad", Expression: "exceptions.otherField.size() == 0"}},
		}},
		{name: "string literal is not global reference", spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "'exceptions' == 'exceptions'"}},
		}, want: true},
		{name: "locally scoped exceptions is not global", spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "object.items.all(exceptions, exceptions != null)"}},
		}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := CanGenerateNativeValidatingPolicyExpressions(
				&policiesv1beta1.ValidatingPolicy{Spec: tt.spec}, tt.exceptions)
			assert.Equal(t, tt.want, ok)
			if !tt.want {
				assert.Assert(t, reason != "")
			}
		})
	}
}
