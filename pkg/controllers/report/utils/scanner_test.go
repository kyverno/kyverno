package utils

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
)

var podMatchSpec = policiesv1beta1.ImageValidatingPolicySpec{
	MatchConstraints: &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{"CREATE"},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
	},
}

func newTestScanner(t *testing.T) Scanner {
	t.Helper()
	scheme := runtime.NewScheme()
	assert.NoError(t, kubescheme.AddToScheme(scheme))
	dClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{})
	assert.NoError(t, err)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	return NewScanner(logging.GlobalLogger(), nil, config.NewDefaultConfiguration(false), nil, dClient, nil, nil, nil)
}

func newDeploymentResource() (unstructured.Unstructured, schema.GroupVersionResource) {
	resource := unstructured.Unstructured{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	resource.SetName("test-deploy")
	resource.SetNamespace("test-ns")
	return resource, schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func TestScanResource_NamespacedImageValidatingPolicy(t *testing.T) {
	nivp := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nivp", Namespace: "test-ns", ResourceVersion: "1"},
		Spec:       podMatchSpec,
	}
	policy := engineapi.NewNamespacedImageValidatingPolicy(nivp)

	resource, gvr := newDeploymentResource()
	results := newTestScanner(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Len(t, results, 1, "NamespacedImageValidatingPolicy must not be silently skipped by the scanner")
}

func TestScanResource_ImageValidatingPolicy(t *testing.T) {
	ivp := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ivp", ResourceVersion: "1"},
		Spec:       podMatchSpec,
	}
	policy := engineapi.NewImageValidatingPolicy(ivp)

	resource, gvr := newDeploymentResource()
	results := newTestScanner(t).ScanResource(t.Context(), resource, gvr, "", nil, nil, nil, nil, policy)

	assert.Len(t, results, 1, "ImageValidatingPolicy must not be silently skipped by the scanner")
}
