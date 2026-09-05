package policy

import (
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
