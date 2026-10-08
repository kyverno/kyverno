package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNestedContextScope(t *testing.T) {
	t.Parallel()
	locations := []struct {
		name string
		path string
		rule func([]kyvernov1.ContextEntry) kyvernov1.Rule
	}{
		{
			name: "mutate foreach",
			path: "mutate.foreach[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				return kyvernov1.Rule{Mutation: &kyvernov1.Mutation{ForEachMutation: []kyvernov1.ForEachMutation{{}, {Context: entries}}}}
			},
		},
		{
			name: "nested mutate foreach",
			path: "mutate.foreach[1].foreach[1].foreach[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				item := kyvernov1.ForEachMutation{Context: entries}
				for range 2 {
					item = kyvernov1.ForEachMutation{ForEachMutation: &kyvernov1.ForEachMutationWrapper{Items: []kyvernov1.ForEachMutation{{}, item}}}
				}
				return kyvernov1.Rule{Mutation: &kyvernov1.Mutation{ForEachMutation: []kyvernov1.ForEachMutation{{}, item}}}
			},
		},
		{
			name: "validate foreach",
			path: "validate.foreach[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				return kyvernov1.Rule{Validation: &kyvernov1.Validation{ForEachValidation: []kyvernov1.ForEachValidation{{}, {Context: entries}}}}
			},
		},
		{
			name: "nested validate foreach",
			path: "validate.foreach[1].foreach[1].foreach[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				item := kyvernov1.ForEachValidation{Context: entries}
				for range 2 {
					item = kyvernov1.ForEachValidation{ForEachValidation: &kyvernov1.ForEachValidationWrapper{Items: []kyvernov1.ForEachValidation{{}, item}}}
				}
				return kyvernov1.Rule{Validation: &kyvernov1.Validation{ForEachValidation: []kyvernov1.ForEachValidation{{}, item}}}
			},
		},
		{
			name: "generate foreach",
			path: "generate.foreach[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				return kyvernov1.Rule{Generation: &kyvernov1.Generation{ForEachGeneration: []kyvernov1.ForEachGeneration{{}, {Context: entries}}}}
			},
		},
		{
			name: "mutate target",
			path: "mutate.targets[1].context[1]",
			rule: func(entries []kyvernov1.ContextEntry) kyvernov1.Rule {
				return kyvernov1.Rule{Mutation: &kyvernov1.Mutation{Targets: []kyvernov1.TargetResourceSpec{{}, {Context: entries}}}}
			},
		},
	}

	imageEntry := func(secret string) kyvernov1.ContextEntry {
		return kyvernov1.ContextEntry{
			Name: "image",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:                "ghcr.io/example/app:latest",
				ImageRegistryCredentials: &kyvernov1.ImageRegistryCredentials{Secrets: []string{secret}},
			},
		}
	}
	mixedImageEntry := imageEntry("tenant-b/pull-secret")
	mixedImageEntry.Variable = &kyvernov1.Variable{}
	mixedGlobalEntry := imageEntry("pull-secret")
	mixedGlobalEntry.GlobalReference = &kyvernov1.GlobalContextEntryReference{Name: "cluster-data"}
	tests := []struct {
		name          string
		entry         kyvernov1.ContextEntry
		namespace     string
		errorContains string
		errorField    string
	}{
		{name: "bare secret", namespace: "tenant-a", entry: imageEntry("pull-secret")},
		{name: "same namespace", namespace: "tenant-a", entry: imageEntry("tenant-a/pull-secret")},
		{name: "foreign namespace", namespace: "tenant-a", entry: imageEntry("tenant-b/pull-secret"), errorContains: "instead of policy namespace", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "installation namespace", namespace: "tenant-a", entry: imageEntry("kyverno/pull-secret"), errorContains: "instead of policy namespace", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "malformed secret", namespace: "tenant-a", entry: imageEntry("tenant-a/pull/extra"), errorContains: "invalid name", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "literal context secret", namespace: "tenant-a", entry: imageEntry("{{ request.object.metadata.name }}"), errorContains: "invalid name", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "missing namespace", entry: imageEntry("pull-secret"), errorContains: "policy namespace must not be empty", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "missing namespace without credentials", entry: kyvernov1.ContextEntry{ImageRegistry: &kyvernov1.ImageRegistry{}}, errorContains: "policy namespace must not be empty", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "missing explicit credentials", namespace: "tenant-a", entry: kyvernov1.ContextEntry{ImageRegistry: &kyvernov1.ImageRegistry{}}},
		{name: "global reference", namespace: "tenant-a", entry: kyvernov1.ContextEntry{GlobalReference: &kyvernov1.GlobalContextEntryReference{Name: "cluster-data"}}, errorContains: "globalReference is not allowed in namespaced policies", errorField: "globalReference"},
		{name: "image reference with multiple sources", namespace: "tenant-a", entry: mixedImageEntry, errorContains: "instead of policy namespace", errorField: "imageRegistry.imageRegistryCredentials"},
		{name: "global reference with multiple sources", namespace: "tenant-a", entry: mixedGlobalEntry, errorContains: "globalReference is not allowed in namespaced policies", errorField: "globalReference"},
		{name: "unrelated context syntax", namespace: "tenant-a", entry: kyvernov1.ContextEntry{Variable: &kyvernov1.Variable{JMESPath: "["}}},
		{name: "configmap scope unchanged", namespace: "tenant-a", entry: kyvernov1.ContextEntry{ConfigMap: &kyvernov1.ConfigMapReference{Name: "settings", Namespace: "tenant-b"}}},
	}
	for _, location := range locations {
		t.Run(location.name, func(t *testing.T) {
			t.Parallel()
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					// An unrelated entry precedes the tested source to prove that
					// every entry is visited without adding context syntax checks.
					entries := []kyvernov1.ContextEntry{{Variable: &kyvernov1.Variable{}}, *test.entry.DeepCopy()}
					rule := location.rule(entries)
					before := rule.DeepCopy()
					err := validateNestedContextScope(rule, true, test.namespace)
					if test.errorContains != "" {
						require.ErrorContains(t, err, test.errorContains)
						assert.Contains(t, err.Error(), location.path+"."+test.errorField)
					} else {
						require.NoError(t, err)
					}
					assert.Equal(t, before, &rule, "scope validation must not rewrite the policy")
					// ClusterPolicy context permissions are unchanged, including
					// installation defaults and explicit foreign references.
					require.NoError(t, validateNestedContextScope(rule, false, ""))
					assert.Equal(t, before, &rule)
				})
			}
		})
	}
}

func TestValidateNestedContextScopeChecksAllRuleTypes(t *testing.T) {
	t.Parallel()
	rule := kyvernov1.Rule{
		Mutation:   &kyvernov1.Mutation{ForEachMutation: []kyvernov1.ForEachMutation{{}}},
		Validation: &kyvernov1.Validation{ForEachValidation: []kyvernov1.ForEachValidation{{}}},
		Generation: &kyvernov1.Generation{ForEachGeneration: []kyvernov1.ForEachGeneration{{Context: []kyvernov1.ContextEntry{{
			GlobalReference: &kyvernov1.GlobalContextEntryReference{Name: "cluster-data"},
		}}}}},
	}
	require.ErrorContains(t, validateNestedContextScope(rule, true, "tenant-a"), "generate.foreach[0].context[0].globalReference")
}
