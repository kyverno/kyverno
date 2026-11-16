package policy

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

func TestTriggerMatchesExcludeRules(t *testing.T) {
	pc := &policyController{log: logr.Discard()}
	pod := unstructured.Unstructured{}
	pod.SetName("skip-me")
	pod.SetNamespace("default")
	pod.SetKind("Pod")

	pods := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podStatus := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/status"}

	tests := []struct {
		name    string
		gvr     schema.GroupVersionResource
		exclude admissionregistrationv1.NamedRuleWithOperations
		want    bool
	}{
		{
			name:    "exact exclude drops the named pod",
			gvr:     pods,
			exclude: excludeRule([]string{""}, []string{"v1"}, []string{"pods"}, []string{"skip-me"}),
			want:    false,
		},
		{
			name:    "wildcard group and version drop the named pod",
			gvr:     pods,
			exclude: excludeRule([]string{"*"}, []string{"*"}, []string{"pods"}, []string{"skip-me"}),
			want:    false,
		},
		{
			name:    "wildcard exclude keeps a different name",
			gvr:     pods,
			exclude: excludeRule([]string{"*"}, []string{"*"}, []string{"pods"}, []string{"other"}),
			want:    true,
		},
		{
			name:    "star resource drops pods",
			gvr:     pods,
			exclude: excludeRule([]string{"*"}, []string{"*"}, []string{"*"}, nil),
			want:    false,
		},
		{
			name:    "star resource does not drop a subresource",
			gvr:     podStatus,
			exclude: excludeRule([]string{""}, []string{"v1"}, []string{"*"}, nil),
			want:    true,
		},
		{
			name:    "pods star drops pod status",
			gvr:     podStatus,
			exclude: excludeRule([]string{""}, []string{"v1"}, []string{"pods/*"}, nil),
			want:    false,
		},
		{
			name:    "pods star drops the pod",
			gvr:     pods,
			exclude: excludeRule([]string{""}, []string{"v1"}, []string{"pods/*"}, nil),
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pc.triggerMatches(pod, tc.gvr, nil, []admissionregistrationv1.NamedRuleWithOperations{tc.exclude}, nil)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetGpolTriggersExcludeWildcard(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name:             "",
				Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "pods", Namespaced: true, Kind: "Pod"}},
			},
		},
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	skip := podNamed("skip-me")
	keep := podNamed("keep-me")
	fakeClient, err := dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "PodList"}, skip, keep)
	require.NoError(t, err)

	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvr})
	disco.AddGVRToGVKMapping(gvr, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}
	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
		ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			excludeRule([]string{"*"}, []string{"*"}, []string{"pods"}, []string{"skip-me"}),
		},
	}

	triggers := pc.getGpolTriggers(match)
	require.Len(t, triggers, 1)
	assert.Equal(t, "keep-me", triggers[0].GetName())
}

func excludeRule(groups, versions, resources, names []string) admissionregistrationv1.NamedRuleWithOperations {
	return admissionregistrationv1.NamedRuleWithOperations{
		ResourceNames: names,
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   groups,
				APIVersions: versions,
				Resources:   resources,
			},
		},
	}
}

func podNamed(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
		},
	}}
}
