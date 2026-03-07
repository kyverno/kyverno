package generate

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
)

func Test_getDownstreams_ForEachProcessesAllEntries(t *testing.T) {
	rule := kyvernov1.Rule{
		Generation: &kyvernov1.Generation{
			ForEachGeneration: []kyvernov1.ForEachGeneration{
				{
					GeneratePattern: kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "foreach-cm-1",
						},
					},
				},
				{
					GeneratePattern: kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "foreach-secret-2",
						},
					},
				},
			},
		},
	}

	ruleContext := kyvernov2.RuleContext{
		Trigger: kyvernov1.ResourceSpec{
			APIVersion: "v1",
			Kind:       "Namespace",
			Namespace:  "trigger-ns",
			Name:       "trigger-name",
			UID:        types.UID("test-uid-123"),
		},
	}

	// Only create a downstream Secret matching the UID-based selector.
	// If selector mutation leaked from the first foreach iteration, the UID label would
	// be missing and this Secret would not be found.
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      "downstream-secret",
			"namespace": "default",
			"labels": map[string]any{
				common.GenerateTriggerUIDLabel:     string(ruleContext.Trigger.GetUID()),
				common.GenerateTriggerNSLabel:      ruleContext.Trigger.GetNamespace(),
				common.GenerateTriggerKindLabel:    ruleContext.Trigger.GetKind(),
				common.GenerateTriggerGroupLabel:   "",
				common.GenerateTriggerVersionLabel: "v1",
			},
		},
	}}
	c := &GenerateController{log: testLogger(t)}
	selector := map[string]string{}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		{Version: "v1", Resource: "secrets"}:    "SecretList",
	}
	fakeClient, err := dclient.NewFakeClient(kubescheme.Scheme, gvrToListKind, []runtime.Object{secret}...)
	assert.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	c.client = fakeClient

	downstreams, err := c.getDownstreams(rule, selector, &ruleContext)
	assert.NoError(t, err)
	if assert.Len(t, downstreams, 1) {
		assert.Equal(t, "Secret", downstreams[0].GetKind())
		assert.Equal(t, "downstream-secret", downstreams[0].GetName())
	}
}

// testLogger returns a no-op logr.Logger for testing
func testLogger(t *testing.T) logr.Logger {
	t.Helper()
	return logr.Discard()
}
