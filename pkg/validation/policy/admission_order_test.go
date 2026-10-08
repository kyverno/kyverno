package policy

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const orderUnknownKind = "MissingReviewResource"

type orderDiscovery struct {
	dclient.IDiscovery
	cached discovery.CachedDiscoveryInterface
}

func (d orderDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

// Exercise production discovery and warning handling, rather than the mock
// validation path which bypasses unknown-kind warnings entirely.
func orderClient() dclient.Interface {
	resources := &discoveryfake.FakeDiscovery{Fake: &k8stesting.Fake{Resources: []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "Pod", Namespaced: true},
			{Name: "configmaps", Kind: "ConfigMap", Namespaced: true},
			{Name: "namespaces", Kind: "Namespace"},
		},
	}}}}
	disco := orderDiscovery{
		IDiscovery: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}}),
		cached:     memory.NewMemCacheClient(resources),
	}
	return dclient.NewFakeClientWithDisco(nil, kubefake.NewClientset(), disco)
}

func orderPolicy(rules ...kyvernov1.Rule) *kyvernov1.Policy {
	return &kyvernov1.Policy{
		TypeMeta: metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "scope-before-warnings", Namespace: "tenant-a",
			Annotations: map[string]string{"pod-policies.kyverno.io/autogen-controllers": "none"},
		},
		Spec: kyvernov1.Spec{Rules: rules},
	}
}

func orderValidationRule(name string) kyvernov1.Rule {
	validation := &kyvernov1.Validation{}
	validation.SetPattern(map[string]interface{}{"metadata": map[string]interface{}{"name": "?*"}})
	return kyvernov1.Rule{
		Name: name,
		MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
			ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
		}}},
		Validation: validation,
	}
}

func orderVerifyRule(secret string) kyvernov1.Rule {
	rule := orderValidationRule("verify-image")
	rule.Validation = nil
	rule.VerifyImages = []kyvernov1.ImageVerification{{
		ImageReferences: []string{"registry.example/*"}, Required: true,
		ImageRegistryCredentials: &kyvernov1.ImageRegistryCredentials{Secrets: []string{secret}},
	}}
	return rule
}

func orderContextRule(entry kyvernov1.ContextEntry) kyvernov1.Rule {
	rule := orderValidationRule("context-check")
	rule.Context = []kyvernov1.ContextEntry{entry}
	return rule
}

func orderImageContext(secret string) kyvernov1.ContextEntry {
	return kyvernov1.ContextEntry{
		Name: "image",
		ImageRegistry: &kyvernov1.ImageRegistry{
			Reference:                "registry.example/app:latest",
			ImageRegistryCredentials: &kyvernov1.ImageRegistryCredentials{Secrets: []string{secret}},
		},
	}
}

func orderGlobalContext() kyvernov1.ContextEntry {
	return kyvernov1.ContextEntry{Name: "global", GlobalReference: &kyvernov1.GlobalContextEntryReference{Name: "tenant-data"}}
}

func orderKindLocations() map[string]func(*kyvernov1.Rule) {
	return map[string]func(*kyvernov1.Rule){
		"match.any": func(rule *kyvernov1.Rule) {
			rule.MatchResources = kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod", orderUnknownKind}},
			}}}
		},
		"match.all": func(rule *kyvernov1.Rule) {
			rule.MatchResources = kyvernov1.MatchResources{All: kyvernov1.ResourceFilters{{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod", orderUnknownKind}},
			}}}
		},
		"match.legacy-kinds": func(rule *kyvernov1.Rule) {
			rule.MatchResources = kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod", orderUnknownKind}}}
		},
		"exclude.any": func(rule *kyvernov1.Rule) {
			rule.ExcludeResources = &kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{orderUnknownKind}},
			}}}
		},
		"exclude.all": func(rule *kyvernov1.Rule) {
			rule.ExcludeResources = &kyvernov1.MatchResources{All: kyvernov1.ResourceFilters{{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{orderUnknownKind}},
			}}}
		},
		"exclude.legacy-kinds": func(rule *kyvernov1.Rule) {
			rule.ExcludeResources = &kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{orderUnknownKind}}}
		},
	}
}

func TestValidate_ScopeChecksPrecedeKindWarnings(t *testing.T) {
	t.Parallel()
	locations := orderKindLocations()
	locations["known Pod"] = func(*kyvernov1.Rule) {}
	for _, test := range []struct {
		name string
		rule kyvernov1.Rule
		err  string
	}{
		{name: "verify foreign", rule: orderVerifyRule("tenant-b/regcred"), err: "instead of policy namespace"},
		{name: "verify installation", rule: orderVerifyRule("kyverno/regcred"), err: "instead of policy namespace"},
		{name: "context foreign", rule: orderContextRule(orderImageContext("tenant-b/regcred")), err: "instead of policy namespace"},
		{name: "context installation", rule: orderContextRule(orderImageContext("kyverno/regcred")), err: "instead of policy namespace"},
		{name: "global reference", rule: orderContextRule(orderGlobalContext()), err: "globalReference is not allowed"},
	} {
		for location, configure := range locations {
			t.Run(test.name+"/"+location, func(t *testing.T) {
				t.Parallel()
				rule := *test.rule.DeepCopy()
				configure(&rule)
				_, err := Validate(orderPolicy(rule), nil, orderClient(), false, "", "")
				require.ErrorContains(t, err, test.err)
				assert.Contains(t, err.Error(), "spec.rules[0]")
			})
		}
	}
}

func TestValidate_UnknownKindInEarlierRuleCannotHideUnsafeLaterRule(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		rule kyvernov1.Rule
		err  string
	}{
		{name: "verify credentials", rule: orderVerifyRule("tenant-b/regcred"), err: "instead of policy namespace"},
		{name: "image context", rule: orderContextRule(orderImageContext("kyverno/regcred")), err: "instead of policy namespace"},
		{name: "global context", rule: orderContextRule(orderGlobalContext()), err: "globalReference is not allowed"},
		{name: "validation foreach context", rule: orderForEachContextRule("validate"), err: "globalReference is not allowed"},
		{name: "mutation foreach context", rule: orderForEachContextRule("mutate"), err: "globalReference is not allowed"},
		{name: "generation foreach context", rule: orderForEachContextRule("generate"), err: "globalReference is not allowed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			earlier := orderValidationRule("unknown-kind")
			orderKindLocations()["match.any"](&earlier)
			_, err := Validate(orderPolicy(earlier, *test.rule.DeepCopy()), nil, orderClient(), false, "", "")
			require.ErrorContains(t, err, test.err)
			assert.Contains(t, err.Error(), "spec.rules[1]")
		})
	}
}

func TestValidate_AutogenCannotHideSubmittedRuleScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		rule kyvernov1.Rule
		err  string
	}{
		{name: "verify credentials", rule: orderVerifyRule("tenant-b/regcred"), err: "instead of policy namespace"},
		{name: "image context", rule: orderContextRule(orderImageContext("tenant-b/regcred")), err: "instead of policy namespace"},
		{name: "global context", rule: orderContextRule(orderGlobalContext()), err: "globalReference is not allowed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rule := *test.rule.DeepCopy()
			rule.Name = "autogen-hidden"
			policy := orderPolicy(orderValidationRule("pod-check"), rule)
			// Enable default autogen so the submitted autogen-* rule is omitted
			// from computed rules, while remaining available to Pod evaluation.
			policy.Annotations = nil
			_, err := Validate(policy, nil, orderClient(), false, "", "")
			require.ErrorContains(t, err, test.err)
			assert.Contains(t, err.Error(), "spec.rules[1]")
		})
	}
}

func orderForEachContextRule(operation string) kyvernov1.Rule {
	rule := orderValidationRule("foreach-context")
	context := []kyvernov1.ContextEntry{orderGlobalContext()}
	switch operation {
	case "validate":
		foreach := kyvernov1.ForEachValidation{List: "request.object.spec.containers", Context: context}
		foreach.SetPattern(map[string]interface{}{"name": "?*"})
		rule.Validation = &kyvernov1.Validation{ForEachValidation: []kyvernov1.ForEachValidation{foreach}}
	case "mutate":
		foreach := kyvernov1.ForEachMutation{List: "request.object.spec.containers", Context: context}
		foreach.SetPatchStrategicMerge(map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"review": "true"}}})
		rule.Validation = nil
		rule.Mutation = &kyvernov1.Mutation{ForEachMutation: []kyvernov1.ForEachMutation{foreach}}
	case "generate":
		foreach := kyvernov1.ForEachGeneration{
			List: "request.object.spec.containers", Context: context,
			GeneratePattern: orderGenerateRule("generated").Generation.GeneratePattern,
		}
		rule.Validation = nil
		rule.Generation = &kyvernov1.Generation{ForEachGeneration: []kyvernov1.ForEachGeneration{foreach}}
	}
	return rule
}

func orderGenerateRule(target string) kyvernov1.Rule {
	rule := orderValidationRule("generate-config")
	rule.Validation = nil
	rule.Generation = &kyvernov1.Generation{
		Synchronize: true,
		GeneratePattern: kyvernov1.GeneratePattern{ResourceSpec: kyvernov1.ResourceSpec{
			APIVersion: "v1", Kind: "ConfigMap", Name: target, Namespace: "tenant-a",
		}},
	}
	rule.Generation.SetData(map[string]interface{}{"data": map[string]interface{}{"review": "true"}})
	return rule
}

func TestValidate_GenerateUpdateWarningCannotHideUnsafeCredentials(t *testing.T) {
	t.Parallel()
	for _, secret := range []string{"tenant-a/regcred", "tenant-b/regcred", "kyverno/regcred"} {
		t.Run(secret, func(t *testing.T) {
			t.Parallel()
			oldPolicy := orderPolicy(orderGenerateRule("old-target"), orderVerifyRule("tenant-a/regcred"))
			newPolicy := orderPolicy(orderGenerateRule("new-target"), orderVerifyRule(secret))
			warnings, err := Validate(newPolicy, oldPolicy, orderClient(), false, "", "")
			if secret == "tenant-a/regcred" {
				require.NoError(t, err)
				assert.Contains(t, strings.Join(warnings, "\n"), "no synchronization will be performed")
			} else {
				require.ErrorContains(t, err, "instead of policy namespace")
				assert.Contains(t, err.Error(), "spec.rules[1]")
			}
		})
	}
}

func TestValidate_ValidCredentialScopesPreserveUnknownKindWarnings(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, secret string
		cluster      bool
	}{
		{name: "namespaced bare", secret: "regcred"},
		{name: "namespaced explicit", secret: "tenant-a/regcred"},
		{name: "cluster foreign", secret: "tenant-b/regcred", cluster: true},
	} {
		for location, configure := range orderKindLocations() {
			t.Run(test.name+"/"+location, func(t *testing.T) {
				t.Parallel()
				rule := orderVerifyRule(test.secret)
				configure(&rule)
				namespaced := orderPolicy(rule)
				var policy kyvernov1.PolicyInterface = namespaced
				if test.cluster {
					policy = &kyvernov1.ClusterPolicy{
						TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
						ObjectMeta: metav1.ObjectMeta{Name: namespaced.Name, Annotations: namespaced.Annotations},
						Spec:       namespaced.Spec,
					}
				}
				warnings, err := Validate(policy, nil, orderClient(), false, "", "")
				require.NoError(t, err)
				assert.Contains(t, strings.Join(warnings, "\n"), orderUnknownKind)
			})
		}
	}
}
