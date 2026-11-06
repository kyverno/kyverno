package policy

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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

// newFakeRESTMapper builds APIGroupResources by hand instead of hitting a
// live cluster's discovery API. It mimics how policyController builds
// pc.restMapper in production, letting the test
// register a resource under multiple concrete versions and exercise
// KindFor exactly the way the real mapper would.
func newFakeRESTMapper(t *testing.T) *restmapper.APIGroupResources {
	t.Helper()
	return &restmapper.APIGroupResources{
		Group: metav1.APIGroup{
			Name: "acm.services.k8s.aws",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "acm.services.k8s.aws/v1alpha1", Version: "v1alpha1"},
				{GroupVersion: "acm.services.k8s.aws/v1", Version: "v1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "acm.services.k8s.aws/v1", Version: "v1"},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1alpha1": {
				{Name: "certificates", Namespaced: true, Kind: "Certificate"},
			},
			"v1": {
				{Name: "certificates", Namespaced: true, Kind: "Certificate"},
			},
		},
	}
}

func TestGetGpolTriggers_WildcardAPIVersions(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{newFakeRESTMapper(t)}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "acm.services.k8s.aws", Version: "v1alpha1", Resource: "certificates"}: "CertificateList",
		{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"}:       "CertificateList",
	}

	certAV1alpha1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "acm.services.k8s.aws/v1alpha1",
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      "cert-a",
			"namespace": "default",
		},
	}}
	certAV1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "acm.services.k8s.aws/v1",
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      "cert-a",
			"namespace": "default",
		},
	}}
	certBV1alpha1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "acm.services.k8s.aws/v1alpha1",
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      "cert-b",
			"namespace": "default",
		},
	}}
	certBV1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "acm.services.k8s.aws/v1",
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      "cert-b",
			"namespace": "default",
		},
	}}

	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, certAV1alpha1, certAV1, certBV1alpha1, certBV1)
	assert.NoError(t, err)

	// The fake client does not configure discovery automatically.
	// Register both certificate versions so ListResource can find them.
	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		{Group: "acm.services.k8s.aws", Version: "v1alpha1", Resource: "certificates"},
		{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"},
	})
	disco.AddGVRToGVKMapping(
		schema.GroupVersionResource{Group: "acm.services.k8s.aws", Version: "v1alpha1", Resource: "certificates"},
		schema.GroupVersionKind{Group: "acm.services.k8s.aws", Version: "v1alpha1", Kind: "Certificate"},
	)
	disco.AddGVRToGVKMapping(
		schema.GroupVersionResource{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"},
		schema.GroupVersionKind{Group: "acm.services.k8s.aws", Version: "v1", Kind: "Certificate"},
	)
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}

	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"acm.services.k8s.aws"},
						APIVersions: []string{"*"},
						Resources:   []string{"certificates"},
					},
				},
			},
		},
	}

	triggers := pc.getGpolTriggers(match)

	// Wildcard must resolve to a single (preferred version) and list once,
	// so each of the 2 distinct objects appears exactly once.
	assert.Len(t, triggers, 2)

	got := map[string]int{}
	for _, trigger := range triggers {
		got[trigger.GetName()]++
		assert.Equal(t, "acm.services.k8s.aws/v1", trigger.GetAPIVersion())
	}
	assert.Equal(t, 1, got["cert-a"])
	assert.Equal(t, 1, got["cert-b"])
}

func TestGetGpolTriggers_SubresourceTrigger(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name:             "",
				Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "pods", Namespaced: true, Kind: "Pod"},
				},
			},
		},
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	scheme := runtime.NewScheme()
	gvrPods := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvrToListKind := map[schema.GroupVersionResource]string{
		gvrPods: "PodList",
	}

	pod := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "my-pod",
			"namespace": "default",
		},
	}}

	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, pod)
	assert.NoError(t, err)

	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvrPods})
	disco.AddGVRToGVKMapping(
		gvrPods,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
	)
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}

	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods/status"},
					},
				},
			},
		},
	}

	triggers := pc.getGpolTriggers(match)
	assert.Len(t, triggers, 1)
	assert.Equal(t, "status", triggers[0].SubResource)
}

func TestGetGpolTriggers_WildcardAPIVersions_Subresource(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name:             "",
				Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "pods", Namespaced: true, Kind: "Pod"},
				},
			},
		},
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	scheme := runtime.NewScheme()
	gvrPods := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvrToListKind := map[schema.GroupVersionResource]string{
		gvrPods: "PodList",
	}

	pod := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "my-pod",
			"namespace": "default",
		},
	}}

	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, pod)
	assert.NoError(t, err)

	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvrPods})
	disco.AddGVRToGVKMapping(
		gvrPods,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
	)
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}

	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"*"},
						Resources:   []string{"pods/status"},
					},
				},
			},
		},
	}

	triggers := pc.getGpolTriggers(match)
	assert.Len(t, triggers, 1)
	assert.Equal(t, "status", triggers[0].SubResource)
	assert.Equal(t, "my-pod", triggers[0].GetName())
}

func TestGetGpolTriggers_DeduplicateTriggers(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name:             "",
				Versions:         []metav1.GroupVersionForDiscovery{{GroupVersion: "v1", Version: "v1"}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "pods", Namespaced: true, Kind: "Pod"},
				},
			},
		},
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	scheme := runtime.NewScheme()
	gvrPods := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	gvrToListKind := map[schema.GroupVersionResource]string{
		gvrPods: "PodList",
	}

	pod := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "my-pod",
			"namespace": "default",
		},
	}}

	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, pod)
	assert.NoError(t, err)

	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvrPods})
	disco.AddGVRToGVKMapping(
		gvrPods,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
	)
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}

	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods", "pods/status"},
					},
				},
			},
		},
	}

	triggers := pc.getGpolTriggers(match)
	assert.Len(t, triggers, 1, "expected duplicate triggers from pods and pods/status to be deduplicated")
}

func TestGetGpolTriggers_ConcreteAPIVersionUnaffected(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{newFakeRESTMapper(t)}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"}: "CertificateList",
	}

	cert := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "acm.services.k8s.aws/v1",
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      "cert-stable",
			"namespace": "default",
		},
	}}

	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, cert)
	assert.NoError(t, err)

	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"},
	})
	disco.AddGVRToGVKMapping(
		schema.GroupVersionResource{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"},
		schema.GroupVersionKind{Group: "acm.services.k8s.aws", Version: "v1", Kind: "Certificate"},
	)
	fakeClient.SetDiscovery(disco)

	pc := &policyController{
		client:     fakeClient,
		restMapper: restMapper,
		log:        logr.Discard(),
	}

	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"acm.services.k8s.aws"},
						APIVersions: []string{"v1"},
						Resources:   []string{"certificates"},
					},
				},
			},
		},
	}

	triggers := pc.getGpolTriggers(match)

	assert.Len(t, triggers, 1)
}

func TestHandleGenerateExistingPropagatesSynchronization(t *testing.T) {
	groupResources := []*restmapper.APIGroupResources{newFakeRESTMapper(t)}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	gvr := schema.GroupVersionResource{Group: "acm.services.k8s.aws", Version: "v1", Resource: "certificates"}
	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{gvr})
	disco.AddGVRToGVKMapping(gvr, schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Certificate"})
	certificate := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "acm.services.k8s.aws/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "cert-a",
			"namespace": "team-a",
		},
	}}
	matchConstraints := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"acm.services.k8s.aws"},
					APIVersions: []string{"v1"},
					Resources:   []string{"certificates"},
				},
			},
		}},
	}
	tests := []struct {
		name        string
		namespaced  bool
		synchronize bool
	}{
		{"generating policy synchronization enabled", false, true},
		{"generating policy synchronization disabled", false, false},
		{"namespaced generating policy synchronization enabled", true, true},
		{"namespaced generating policy synchronization disabled", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient, err := dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "CertificateList"}, certificate)
			require.NoError(t, err)
			fakeClient.SetDiscovery(disco)

			generator := &captureURGenerator{}
			controller := &policyController{
				client:      fakeClient,
				restMapper:  restMapper,
				urGenerator: generator,
				log:         logr.Discard(),
			}

			spec := policiesv1beta1.GeneratingPolicySpec{
				MatchConstraints: matchConstraints,
				EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
					SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: new(tc.synchronize)},
				},
			}

			if tc.namespaced {
				err = controller.handleNamespacedGenerateExisting(&policiesv1beta1.NamespacedGeneratingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "team-a"},
					Spec:       spec,
				})
			} else {
				err = controller.handleGenerateExisting(&policiesv1beta1.GeneratingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec:       spec,
				})
			}

			require.NoError(t, err)
			require.Len(t, generator.captured, 1)
			require.Len(t, generator.captured[0].Spec.RuleContext, 1)
			assert.Equal(t, tc.synchronize, generator.captured[0].Spec.RuleContext[0].Synchronize)
		})
	}
}

func TestGetGpolTriggers_WildcardAPIGroups(t *testing.T) {
	const customGroup = "example.test.io"
	coreGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	coreNamespaceGVR := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	customAlphaGVR := schema.GroupVersionResource{Group: customGroup, Version: "v1alpha1", Resource: "configmaps"}
	customV1GVR := schema.GroupVersionResource{Group: customGroup, Version: "v1", Resource: "configmaps"}

	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
					{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: customGroup,
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: customGroup + "/v1alpha1", Version: "v1alpha1"},
					{GroupVersion: customGroup + "/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: customGroup + "/v1",
					Version:      "v1",
				},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1alpha1": {{Name: "configmaps", Namespaced: true, Kind: "ExampleConfigMap"}},
				"v1":       {{Name: "configmaps", Namespaced: true, Kind: "ExampleConfigMap"}},
			},
		},
	})

	newResource := func(apiVersion, kind, name, uid string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": "default",
				"uid":       uid,
			},
		}}
	}
	client, err := dclient.NewFakeClient(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			coreGVR:          "ConfigMapList",
			coreNamespaceGVR: "NamespaceList",
			customAlphaGVR:   "ExampleConfigMapList",
			customV1GVR:      "ExampleConfigMapList",
		},
		newResource("v1", "ConfigMap", "core-config", "core-uid"),
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "Namespace",
			"metadata": map[string]any{"name": "existing-ns", "uid": "namespace-uid"},
		}},
	)
	require.NoError(t, err)

	discovery := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{coreGVR, coreNamespaceGVR, customAlphaGVR, customV1GVR})
	discovery.AddGVRToGVKMapping(coreGVR, coreGVR.GroupVersion().WithKind("ConfigMap"))
	discovery.AddGVRToGVKMapping(coreNamespaceGVR, coreNamespaceGVR.GroupVersion().WithKind("Namespace"))
	discovery.AddGVRToGVKMapping(customAlphaGVR, customAlphaGVR.GroupVersion().WithKind("ExampleConfigMap"))
	discovery.AddGVRToGVKMapping(customV1GVR, customV1GVR.GroupVersion().WithKind("ExampleConfigMap"))
	client.SetDiscovery(discovery)

	// NewFakeClient's object tracker guesses the resource plural from Kind.
	// In this fixture the custom kind deliberately uses "configmaps", so seed
	// through the discovery-backed client to store it under the correct GVR.
	for _, version := range []string{"v1alpha1", "v1"} {
		apiVersion := customGroup + "/" + version
		_, err := client.CreateResource(context.Background(), apiVersion, "ExampleConfigMap", "default",
			newResource(apiVersion, "ExampleConfigMap", "custom-config", "custom-uid"), false)
		require.NoError(t, err)
	}
	for _, version := range []string{"v1alpha1", "v1"} {
		listed, err := client.ListResource(context.Background(), customGroup+"/"+version, "ExampleConfigMap", "", nil)
		require.NoError(t, err)
		require.Len(t, listed.Items, 1)
	}

	pc := &policyController{client: client, restMapper: mapper, log: logr.Discard()}
	namespacedScope := admissionregistrationv1.NamespacedScope
	clusterScope := admissionregistrationv1.ClusterScope
	tests := []struct {
		name                 string
		groups               []string
		versions             []string
		resourceNames        []string
		resource             string
		scope                *admissionregistrationv1.ScopeType
		excludeCore          bool
		excludeAlpha         bool
		excludeStatus        bool
		overlapCore          bool
		wantNames            []string
		wantSubresource      string
		wantCustomAPIVersion string
	}{
		{
			name:   "wildcard group with v1 matches both groups",
			groups: []string{"*"}, versions: []string{"v1"},
			wantNames: []string{"core-config", "custom-config"},
		},
		{
			name:   "wildcard group respects explicit version",
			groups: []string{"*"}, versions: []string{"v1alpha1"},
			wantNames: []string{"custom-config"},
		},
		{
			name:   "wildcard group and version use one preferred version per group",
			groups: []string{"*"}, versions: []string{"*"},
			wantNames:            []string{"core-config", "custom-config"},
			wantCustomAPIVersion: customGroup + "/v1",
		},
		{
			name:   "multiple explicit versions retain eligible matches",
			groups: []string{"*"}, versions: []string{"v1alpha1", "v1"},
			wantNames:            []string{"core-config", "custom-config"},
			wantCustomAPIVersion: customGroup + "/v1alpha1",
		},
		{
			name:   "excluded earlier version does not hide a later match",
			groups: []string{"*"}, versions: []string{"v1alpha1", "v1"},
			excludeAlpha: true, wantNames: []string{"core-config", "custom-config"},
			wantCustomAPIVersion: customGroup + "/v1",
		},
		{
			name:   "concrete core group is unchanged",
			groups: []string{""}, versions: []string{"v1"},
			wantNames: []string{"core-config"},
		},
		{
			name:   "concrete custom group wildcard version is unchanged",
			groups: []string{customGroup}, versions: []string{"*"},
			wantNames: []string{"custom-config"},
		},
		{
			name:   "overlapping wildcard and concrete rules are deduplicated",
			groups: []string{"*"}, versions: []string{"v1"},
			overlapCore: true, wantNames: []string{"core-config", "custom-config"},
		},
		{
			name:   "wildcard group preserves core group exclusions",
			groups: []string{"*"}, versions: []string{"v1"},
			excludeCore: true, wantNames: []string{"custom-config"},
		},
		{
			name:   "wildcard group preserves subresource identity",
			groups: []string{"*"}, versions: []string{"v1"}, resource: "configmaps/status",
			wantNames: []string{"core-config", "custom-config"}, wantSubresource: "status",
		},
		{
			name:   "wildcard group honors subresource exclusion",
			groups: []string{"*"}, versions: []string{"v1"}, resource: "configmaps/status",
			excludeStatus: true,
		},
		{
			name:   "resource names are still respected",
			groups: []string{"*"}, versions: []string{"v1"},
			resourceNames: []string{"core-config"}, wantNames: []string{"core-config"},
		},
		{
			name:   "namespaced scope includes namespaced resources",
			groups: []string{"*"}, versions: []string{"v1"}, scope: &namespacedScope,
			wantNames: []string{"core-config", "custom-config"},
		},
		{
			name:   "cluster scope excludes namespaced resources",
			groups: []string{"*"}, versions: []string{"v1"}, scope: &clusterScope,
		},
		{
			name:   "cluster scope includes cluster-scoped resources",
			groups: []string{"*"}, versions: []string{"v1"}, resource: "namespaces", scope: &clusterScope,
			wantNames: []string{"existing-ns"},
		},
		{
			name:   "namespaced scope excludes cluster-scoped resources",
			groups: []string{"*"}, versions: []string{"v1"}, resource: "namespaces", scope: &namespacedScope,
		},
		{
			name:   "unserved version does not produce a trigger",
			groups: []string{"*"}, versions: []string{"v2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := tc.resource
			if resource == "" {
				resource = "configmaps"
			}
			match := &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					ResourceNames: tc.resourceNames,
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   tc.groups,
							APIVersions: tc.versions,
							Resources:   []string{resource},
							Scope:       tc.scope,
						},
					},
				}},
			}
			if tc.overlapCore {
				// Wildcards must be the sole element of APIGroups. Model overlap
				// with separate valid match rules instead of ["*", ""].
				match.ResourceRules = append(match.ResourceRules, admissionregistrationv1.NamedRuleWithOperations{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				})
			}
			if tc.excludeAlpha {
				match.ExcludeResourceRules = append(match.ExcludeResourceRules, admissionregistrationv1.NamedRuleWithOperations{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{customGroup},
							APIVersions: []string{"v1alpha1"},
							Resources:   []string{"configmaps"},
						},
					},
				})
			}
			if tc.excludeStatus {
				match.ExcludeResourceRules = append(match.ExcludeResourceRules, admissionregistrationv1.NamedRuleWithOperations{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps/status"},
						},
					},
				})
			}
			if tc.excludeCore {
				match.ExcludeResourceRules = []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"*"},
							Resources:   []string{"configmaps"},
						},
					},
				}}
			}
			triggers := pc.getGpolTriggers(match)
			gotNames := make([]string, 0, len(triggers))
			for _, trigger := range triggers {
				gotNames = append(gotNames, trigger.GetName())
				if tc.wantSubresource != "" {
					assert.Equal(t, tc.wantSubresource, trigger.SubResource)
				}
				if trigger.GetName() == "custom-config" {
					if tc.wantCustomAPIVersion != "" {
						assert.Equal(t, tc.wantCustomAPIVersion, trigger.GetAPIVersion())
					}
				}
			}
			assert.ElementsMatch(t, tc.wantNames, gotNames)
		})
	}
	// The base resource and its status subresource represent distinct admission
	// contexts. A mutating policy can accept only status updates, so processing
	// a base rule first must not suppress the status trigger in another version.
	t.Run("base and status contexts remain distinct across API versions", func(t *testing.T) {
		makeRule := func(version, resource string) admissionregistrationv1.NamedRuleWithOperations {
			return admissionregistrationv1.NamedRuleWithOperations{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{customGroup},
						APIVersions: []string{version},
						Resources:   []string{resource},
					},
				},
			}
		}
		match := &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
				makeRule("v1alpha1", "configmaps"),
				makeRule("v1", "configmaps/status"),
				makeRule("v1alpha1", "configmaps/status"),
			},
		}
		triggers := pc.getGpolTriggers(match)
		require.Len(t, triggers, 2)
		versionsBySubresource := make(map[string]string, len(triggers))
		for _, trigger := range triggers {
			assert.Equal(t, "custom-config", trigger.GetName())
			versionsBySubresource[trigger.SubResource] = trigger.GetAPIVersion()
		}
		assert.Equal(t, map[string]string{
			"":       customGroup + "/v1alpha1",
			"status": customGroup + "/v1",
		}, versionsBySubresource)
	})

}
