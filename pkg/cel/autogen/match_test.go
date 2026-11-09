package autogen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestCreateMatchConstraints(t *testing.T) {
	tests := []struct {
		name              string
		targets           []policiesv1beta1.Target
		operations        []admissionregistrationv1.OperationType
		namespaceSelector *metav1.LabelSelector
		want              *admissionregistrationv1.MatchResources
	}{{
		name:       "nil targets",
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want:       nil,
	}, {
		name:       "empty targets",
		targets:    []policiesv1beta1.Target{},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want:       nil,
	}, {
		name: "nil operations",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		want: nil,
	}, {
		name: "empty operations",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{},
		want:       nil,
	}, {
		name: "single target",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}, {
		name: "multiple targets",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}, {
			Group:    "flop",
			Version:  "v1",
			Resource: "foos",
			Kind:     "Foo",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "foos",
			Kind:     "Foo",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v1"},
						Resources:   []string{"foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v2"},
						Resources:   []string{"bars", "foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}, {
		name: "with namespace selector",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		namespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"kubernetes.io/namespace-type": "product"},
		},
		want: &admissionregistrationv1.MatchResources{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"kubernetes.io/namespace-type": "product"},
			},
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateMatchConstraints(tt.targets, tt.operations, tt.namespaceSelector)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateMatchConditions(t *testing.T) {
	tests := []struct {
		name         string
		replacements string
		targets      []policiesv1beta1.Target
		conditions   []admissionregistrationv1.MatchCondition
		want         []admissionregistrationv1.MatchCondition
	}{{
		name: "nil targets",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: nil,
	}, {
		name:    "empty targets",
		targets: []policiesv1beta1.Target{},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: nil,
	}, {
		name: "nil conditions",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		want: nil,
	}, {
		name: "empty conditions",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{},
		want:       []admissionregistrationv1.MatchCondition{},
	}, {
		name: "single target",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-foo",
			Expression: "!((object.apiVersion == 'foo/v1' && object.kind =='Bar')) || (something)",
		}},
	}, {
		name: "multiple targets",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "foos",
			Kind:     "Foo",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-foo",
			Expression: "!((object.apiVersion == 'flop/v2' && object.kind =='Foo') || (object.apiVersion == 'foo/v1' && object.kind =='Bar')) || (something)",
		}},
	}, {
		name:         "with name",
		replacements: "test",
		targets: []policiesv1beta1.Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-test-foo",
			Expression: "!((object.apiVersion == 'foo/v1' && object.kind =='Bar')) || (something)",
		}},
	}, {
		name:         "extraction target omits unrewritten conditions",
		replacements: ExtractionReplacementsRef,
		targets: []policiesv1beta1.Target{{
			Group:    "jobset.x-k8s.io",
			Version:  "v1alpha2",
			Resource: "jobsets",
			Kind:     "JobSet",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "object.spec.containers.exists(c, c.image.endsWith(':latest'))",
		}},
		want: nil,
	}, {
		name:         "long condition name exceeding 63 characters is truncated and hashed",
		replacements: "defaults",
		targets: []policiesv1beta1.Target{{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
			Kind:     "Pod",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "exclude-kube-system-and-monitoring-workloads-canary",
			Expression: "object.metadata.namespace != 'kube-system'",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       formatMatchConditionName("autogen-defaults", "exclude-kube-system-and-monitoring-workloads-canary"),
			Expression: "!((object.apiVersion == 'v1' && object.kind =='Pod')) || (object.metadata.namespace != 'kube-system')",
		}},
	}, {
		name:         "condition expression with single-line comment gets newline before closing parenthesis",
		replacements: "defaults",
		targets: []policiesv1beta1.Target{{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
			Kind:     "Pod",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "check-namespace",
			Expression: "object.metadata.namespace != 'kube-system' // ignore system",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-defaults-check-namespace",
			Expression: "!((object.apiVersion == 'v1' && object.kind =='Pod')) || (object.metadata.namespace != 'kube-system' // ignore system\n)",
		}},
	}, {
		name:         "condition expression with single-line comment already ending in newline",
		replacements: "defaults",
		targets: []policiesv1beta1.Target{{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
			Kind:     "Pod",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "check-namespace",
			Expression: "object.metadata.namespace != 'kube-system' // ignore system\n",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-defaults-check-namespace",
			Expression: "!((object.apiVersion == 'v1' && object.kind =='Pod')) || (object.metadata.namespace != 'kube-system' // ignore system\n)",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateMatchConditions(tt.replacements, tt.targets, tt.conditions)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatMatchConditionName(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		condName string
	}{
		{
			name:     "short name",
			prefix:   "autogen",
			condName: "foo",
		},
		{
			name:     "with config prefix",
			prefix:   "autogen-defaults",
			condName: "check-namespace",
		},
		{
			name:     "exactly 63 characters",
			prefix:   "autogen",
			condName: strings.Repeat("a", 63-len("autogen-")),
		},
		{
			name:     "64 characters exceeds limit by 1",
			prefix:   "autogen",
			condName: strings.Repeat("a", 64-len("autogen-")),
		},
		{
			name:     "long condition name 70 characters",
			prefix:   "autogen-defaults",
			condName: "exclude-kube-system-and-monitoring-workloads-canary",
		},
		{
			name:   "boundary hyphen at truncation point",
			prefix: "autogen-defaults",
			// 54th character lands on a hyphen
			condName: strings.Repeat("a", 54-len("autogen-defaults-")-1) + "-suffix-that-will-be-truncated",
		},
		{
			name:     "very long condition name 120 characters",
			prefix:   "autogen-cronjobs",
			condName: strings.Repeat("very-long-name-", 8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMatchConditionName(tt.prefix, tt.condName)

			// Must not exceed 63 characters
			assert.LessOrEqual(t, len(result), 63, fmt.Sprintf("formatted name %q exceeds 63 characters: len=%d", result, len(result)))

			// Must be a valid Kubernetes qualified name
			errs := validation.IsQualifiedName(result)
			assert.Empty(t, errs, fmt.Sprintf("formatted name %q is not a valid qualified name: %v", result, errs))

			// Must be deterministic
			result2 := formatMatchConditionName(tt.prefix, tt.condName)
			assert.Equal(t, result, result2)
		})
	}

	t.Run("collision avoidance for shared prefix", func(t *testing.T) {
		name1 := formatMatchConditionName("autogen-defaults", "shared-prefix-that-is-long-enough-to-be-truncated-variant-1")
		name2 := formatMatchConditionName("autogen-defaults", "shared-prefix-that-is-long-enough-to-be-truncated-variant-2")
		assert.NotEqual(t, name1, name2)
		assert.LessOrEqual(t, len(name1), 63)
		assert.LessOrEqual(t, len(name2), 63)
		assert.Empty(t, validation.IsQualifiedName(name1))
		assert.Empty(t, validation.IsQualifiedName(name2))
	})
}

func TestCELCompilationWithTrailingComment(t *testing.T) {
	targets := []policiesv1beta1.Target{{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
		Kind:     "Pod",
	}}
	conditions := []admissionregistrationv1.MatchCondition{{
		Name:       "exclude-system",
		Expression: "object.metadata.namespace != 'kube-system' // do not run on kube-system",
	}}

	result := CreateMatchConditions("defaults", targets, conditions)
	require.Len(t, result, 1)

	// Verify CEL compiles without syntax errors (e.g., missing ')' at '<EOF>')
	env, err := cel.NewEnv(
		cel.Variable("object", cel.MapType(cel.StringType, cel.DynType)),
	)
	require.NoError(t, err)

	ast, issues := env.Compile(result[0].Expression)
	require.NoError(t, issues.Err(), fmt.Sprintf("CEL expression failed to compile: %s", result[0].Expression))
	require.NotNil(t, ast)
}
