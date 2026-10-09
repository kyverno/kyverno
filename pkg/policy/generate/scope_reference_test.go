package generate_test

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestNamespacedGenerateReferenceCannotCopyDynamicNameIntoKind(t *testing.T) {
	t.Parallel()

	for _, reference := range []string{"$(./../name)", "$(/name)"} {
		for _, foreach := range []bool{false, true} {
			name := "direct/" + reference
			if foreach {
				name = "foreach/" + reference
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				pattern := kyvernov1.GeneratePattern{
					ResourceSpec: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       reference,
						Name:       "{{request.object.metadata.labels.target}}",
						Namespace:  "tenant",
					},
				}
				pattern.SetData(map[string]interface{}{"data": map[string]interface{}{"source": "policy"}})

				ctx := enginecontext.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
				assert.NilError(t, ctx.AddResource(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"target": "ConfigMap"},
					},
				}))
				// The generator substitutes each GeneratePattern independently. A
				// reference first copies the name expression, then variable
				// substitution makes the resource type depend on the trigger.
				resolved, err := variables.SubstituteAllInType(logr.Discard(), ctx, &pattern)
				assert.NilError(t, err)
				assert.Equal(t, resolved.Kind, "ConfigMap")
				assert.Equal(t, resolved.Name, "ConfigMap")

				generation := &kyvernov1.Generation{GeneratePattern: pattern}
				wantField := "spec.rules[0]"
				if foreach {
					generation = &kyvernov1.Generation{
						ForEachGeneration: []kyvernov1.ForEachGeneration{{
							List:            "request.object.spec.containers",
							GeneratePattern: pattern,
						}},
					}
					wantField += ".foreach[0]"
				}
				wantField += ".kind"
				policy := &kyvernov1.Policy{
					ObjectMeta: metav1.ObjectMeta{Name: "generate-reference", Namespace: "tenant"},
					Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
						Name: "generate-resource",
						MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
							ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
						}}},
						Generation: generation,
					}}},
				}
				_, validationErrors := policy.Validate(sets.New[string]())
				foundScopeError := false
				for _, validationError := range validationErrors {
					if validationError.Type == field.ErrorTypeForbidden && validationError.Field == wantField && strings.Contains(validationError.Detail, "references") {
						foundScopeError = true
					}
				}
				assert.Assert(t, foundScopeError, "expected reference scope error at %s, got %v", wantField, validationErrors)

				// The CLI uses mock validation without discovery or live RBAC.
				// The namespaced scope guard must reject the original policy there
				// before any runtime reference or variable substitution.
				_, err = policyvalidation.Validate(policy, nil, nil, true, "", "")
				assert.ErrorContains(t, err, wantField+": Forbidden")
				assert.ErrorContains(t, err, "references")
			})
		}
	}
}
