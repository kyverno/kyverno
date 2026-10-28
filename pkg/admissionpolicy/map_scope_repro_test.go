package admissionpolicy_test

import (
	"encoding/json"
	"reflect"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	nativematching "k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type scopeCriteria struct {
	constraints admissionregistrationv1.MatchResources
}

func (c scopeCriteria) GetMatchResources() admissionregistrationv1.MatchResources {
	return c.constraints
}

func (c scopeCriteria) GetParsedNamespaceSelector() (labels.Selector, error) {
	if c.constraints.NamespaceSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(c.constraints.NamespaceSelector)
}

func (c scopeCriteria) GetParsedObjectSelector() (labels.Selector, error) {
	if c.constraints.ObjectSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(c.constraints.ObjectSelector)
}

// TestGeneratedMAPScope uses production builders and Kubernetes' native
// admission-policy matcher. It does not require a running API server.
func TestGeneratedMAPScope(t *testing.T) {
	exact := admissionregistrationv1.Exact
	rule := admissionregistrationv1.NamedRuleWithOperations{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
			},
		},
	}
	excluded := rule.DeepCopy()
	excluded.ResourceNames = []string{"do-not-touch"}
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "scope-repro", UID: "policy-uid"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules:        []admissionregistrationv1.NamedRuleWithOperations{rule},
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{*excluded},
				NamespaceSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"environment": "development"}},
				ObjectSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"mutate": "yes"}},
				MatchPolicy:          &exact,
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op: "add", path: "/data/managed", value: "true"}]`},
			}},
		},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, ns := range []*corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "dev", Labels: map[string]string{"environment": "development"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "prod", Labels: map[string]string{"environment": "production"}}},
	} {
		if err := indexer.Add(ns); err != nil {
			t.Fatal(err)
		}
	}
	matcher := nativematching.NewMatcher(corelisters.NewNamespaceLister(indexer), nil)
	builders := map[string]func() any{
		"v1": func() any {
			p := &admissionregistrationv1.MutatingAdmissionPolicy{}
			admissionpolicy.BuildMutatingAdmissionPolicyV1(p, mp, nil)
			return p.Spec.MatchConstraints
		},
		"v1beta1": func() any {
			p := &admissionregistrationv1beta1.MutatingAdmissionPolicy{}
			admissionpolicy.BuildMutatingAdmissionPolicyBeta(p, mp, nil)
			return p.Spec.MatchConstraints
		},
		"v1alpha1": func() any {
			p := &admissionregistrationv1alpha1.MutatingAdmissionPolicy{}
			admissionpolicy.BuildMutatingAdmissionPolicy(p, mp, nil)
			return p.Spec.MatchConstraints
		},
	}
	for version, build := range builders {
		t.Run(version, func(t *testing.T) {
			// MatchResources has the same JSON shape in all three native APIs.
			built := build()
			serialized, err := json.Marshal(built)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("generated matchConstraints: %s", serialized)
			var generated admissionregistrationv1.MatchResources
			if err := json.Unmarshal(serialized, &generated); err != nil {
				t.Fatal(err)
			}
			t.Run("preserves_fields", func(t *testing.T) {
				if !reflect.DeepEqual(*mp.Spec.MatchConstraints, generated) {
					t.Error("generated matchConstraints differ from source policy")
				}
			})
			for _, tc := range []struct {
				name, namespace, objectName, kind, resource, label string
				want                                               bool
			}{
				{"selected_control", "dev", "selected", "ConfigMap", "configmaps", "yes", true},
				{"namespace_excluded", "prod", "selected", "ConfigMap", "configmaps", "yes", false},
				{"object_excluded", "dev", "selected", "ConfigMap", "configmaps", "no", false},
				{"name_excluded", "dev", "do-not-touch", "ConfigMap", "configmaps", "yes", false},
				{"unrelated_kind_control", "dev", "selected", "Secret", "secrets", "yes", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: tc.objectName, Namespace: tc.namespace, Labels: map[string]string{"mutate": tc.label}}}
					attr := admission.NewAttributesRecord(obj, nil,
						schema.GroupVersionKind{Version: "v1", Kind: tc.kind}, tc.namespace, tc.objectName,
						schema.GroupVersionResource{Version: "v1", Resource: tc.resource}, "", admission.Create, nil, false, nil)
					sourceMatch, _, _, err := matcher.Matches(attr, nil, scopeCriteria{*mp.Spec.MatchConstraints})
					if err != nil || sourceMatch != tc.want {
						t.Fatalf("invalid source fixture: match=%v want=%v err=%v", sourceMatch, tc.want, err)
					}
					generatedMatch, _, _, err := matcher.Matches(attr, nil, scopeCriteria{generated})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("source_matches=%v generated_matches=%v", sourceMatch, generatedMatch)
					if generatedMatch != sourceMatch {
						t.Errorf("native generation changed policy scope: source=%v generated=%v", sourceMatch, generatedMatch)
					}
				})
			}
			// The generated selectors must not share mutable maps with the source.
			var selector *metav1.LabelSelector
			switch constraints := built.(type) {
			case *admissionregistrationv1.MatchResources:
				selector = constraints.NamespaceSelector
			case *admissionregistrationv1beta1.MatchResources:
				selector = constraints.NamespaceSelector
			case *admissionregistrationv1alpha1.MatchResources:
				selector = constraints.NamespaceSelector
			default:
				t.Fatalf("unexpected constraint type %T", built)
			}
			selector.MatchLabels["environment"] = "changed"
			if got := mp.Spec.MatchConstraints.NamespaceSelector.MatchLabels["environment"]; got != "development" {
				t.Errorf("builder modified source namespace selector: %q", got)
			}
		})
	}
}
