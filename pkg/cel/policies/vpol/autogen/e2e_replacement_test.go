package autogen

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/sets"
)

// End-to-end checks of the full vpol autogen path (Autogen ->
// generateRuleForControllers -> json.Marshal(whole spec) -> autogen.Apply)
// for matchConditions namespace exclusions. Every spelling must stay
// anchored to the workload object's own metadata in the generated
// controller rules — including spellings as they appear in the marshaled
// JSON the production path actually rewrites.
func TestE2ENamespaceMatchConditions(t *testing.T) {
	tests := []struct {
		name string
		expr string // as written in the policy
	}{
		{
			name: "optional-select form",
			expr: "!(object.metadata.?namespace.orValue('') in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, single quotes",
			expr: "!(object.metadata['namespace'] in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, double quotes",
			expr: "!(object.metadata[\"namespace\"] in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, triple single quotes",
			expr: "!(object.metadata['''namespace'''] in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, triple double quotes",
			expr: "!(object.metadata[\"\"\"namespace\"\"\"] in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, raw string",
			expr: "!(object.metadata[r'namespace'] in ['kube-system', 'kyverno'])",
		},
		{
			name: "bracket form, raw double-quoted string",
			expr: "!(object.metadata[r\"namespace\"] in ['kube-system', 'kyverno'])",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := map[string]any{
				"matchConstraints": map[string]any{
					"resourceRules": []any{map[string]any{
						"apiGroups": []any{""}, "apiVersions": []any{"v1"},
						"operations": []any{"CREATE", "UPDATE"}, "resources": []any{"pods"},
					}},
				},
				"matchConditions": []any{map[string]any{
					"name":       "skip-system-namespaces",
					"expression": tt.expr,
				}},
				"validations": []any{map[string]any{"expression": "object.spec.containers.all(c, true)"}},
			}
			raw, err := json.Marshal(spec)
			assert.NoError(t, err)
			var parsed policiesv1beta1.ValidatingPolicySpec
			assert.NoError(t, json.Unmarshal(raw, &parsed))
			rules, err := generateRuleForControllers(parsed, sets.New("deployments", "cronjobs"))
			assert.NoError(t, err)
			// the generated rules (deployments and cronjobs alike) must
			// carry the expression unchanged: object.metadata is the
			// workload's own metadata and is never moved into the pod
			// template for namespace checks
			for _, config := range []string{"defaults", "cronjobs"} {
				rule, ok := rules[config]
				if !ok {
					t.Fatalf("no generated rule for %s", config)
				}
				if assert.Len(t, rule.Spec.MatchConditions, 1) {
					assert.Equal(t, tt.expr, rule.Spec.MatchConditions[0].Expression)
				}
			}
		})
	}
}
