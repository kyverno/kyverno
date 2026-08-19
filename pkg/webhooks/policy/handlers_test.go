package policy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"
)

// cachedDiscovery wraps a dclient discovery implementation so that
// CachedDiscoveryInterface() returns a working cache. The default dclient fake
// returns nil here, which the policy validator dereferences and panics on.
type cachedDiscovery struct {
	dclient.IDiscovery
	k8s discovery.DiscoveryInterface
}

func (c cachedDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return memory.NewMemCacheClient(c.k8s)
}

func TestPolicyHandlerDeprecationWarnings(t *testing.T) {
	old := metrics.GetManager()
	t.Cleanup(func() { metrics.SetManager(old) })
	metrics.SetManager(metrics.NewMetricsConfigManager(logr.Discard(), kconfig.NewDefaultMetricsConfiguration()))

	client, err := dclient.NewFakeClient(scheme.Scheme, map[schema.GroupVersionResource]string{})
	assert.NoError(t, err)
	fd := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
	})
	client.SetDiscovery(cachedDiscovery{
		IDiscovery: fd,
		k8s:        &fake.FakeDiscovery{Fake: &ktesting.Fake{}},
	})

	h := NewHandlers(client, nil, "", "")

	raw := []byte(`{
		"apiVersion": "kyverno.io/v2beta1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test-cp"},
		"spec": {
			"validationFailureAction": "Enforce",
			"webhookConfiguration": {
				"matchConditions": [{"name": "invalid", "expression": "this is not valid cel !!"}]
			},
			"rules": [{"name": "check", "match": {"any": [{"resources": {"kinds": ["Pod"]}}]}}]
		}
	}`)

	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterPolicy"},
			Object:    runtime.RawExtension{Raw: raw},
		},
		GroupVersionKind: schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterPolicy"},
	}

	resp := h.Validate(context.Background(), logr.Discard(), req, "", time.Time{})
	joined := strings.Join(resp.Warnings, " ")
	// The legacy-kind deprecation (CEL migration) warning is surfaced on every
	// kyverno.io policy, and the deprecated-field warning comes from CheckPolicy.
	assert.Contains(t, joined, "ClusterPolicy (kyverno.io) is deprecated")
	assert.Contains(t, joined, "spec.validationFailureAction is deprecated")
}
