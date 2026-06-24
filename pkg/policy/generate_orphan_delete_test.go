package policy

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type captureURGenerator struct {
	captured []*kyvernov2.UpdateRequest
}

func (g *captureURGenerator) Generate(_ context.Context, _ versioned.Interface, resource *kyvernov2.UpdateRequest, _ logr.Logger) (*kyvernov2.UpdateRequest, error) {
	g.captured = append(g.captured, resource.DeepCopy())
	return nil, nil
}

func buildClonePolicy(orphanDownstreamOnPolicyDelete bool) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clone-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "clone-rule",
					Generation: &kyvernov1.Generation{
						Synchronize:                    true,
						OrphanDownstreamOnPolicyDelete: orphanDownstreamOnPolicyDelete,
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								APIVersion: "v1",
								Kind:       "ConfigMap",
								Namespace:  "default",
								Name:       "target-cm",
							},
							Clone: kyvernov1.CloneFrom{
								Namespace: "default",
								Name:      "source-cm",
							},
						},
					},
				},
			},
		},
	}
}

func buildDownstreamForRule() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetNamespace("default")
	u.SetName("target-cm")
	u.SetUID(types.UID("downstream-uid"))
	u.SetLabels(map[string]string{
		common.GeneratePolicyLabel:          "clone-policy",
		common.GeneratePolicyNamespaceLabel: "",
		common.GenerateRuleLabel:            "clone-rule",
		kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
		common.GenerateTriggerKindLabel:     "Namespace",
		common.GenerateTriggerNSLabel:       "",
		common.GenerateTriggerNameLabel:     "default",
		common.GenerateTriggerUIDLabel:      "trigger-uid",
		common.GenerateTriggerGroupLabel:    "",
		common.GenerateTriggerVersionLabel:  "v1",
	})
	return u
}

func TestCreateURForDownstreamDeletion_CloneRule_OrphanTrueSkipsCleanup(t *testing.T) {
	policy := buildClonePolicy(true)
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, buildDownstreamForRule())
	require.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	generator := &captureURGenerator{}
	controller := &policyController{
		client:      client,
		urGenerator: generator,
		log:         logging.WithName("policy-test"),
	}

	err = controller.createURForDownstreamDeletion(policy)
	require.NoError(t, err)
	assert.Empty(t, generator.captured, "cleanup UR should not be created when orphanDownstreamOnPolicyDelete=true")
}

func TestCreateURForDownstreamDeletion_CloneRule_OrphanFalseCreatesCleanupUR(t *testing.T) {
	policy := buildClonePolicy(false)
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, buildDownstreamForRule())
	require.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	generator := &captureURGenerator{}
	controller := &policyController{
		client:      client,
		urGenerator: generator,
		log:         logging.WithName("policy-test"),
	}

	err = controller.createURForDownstreamDeletion(policy)
	require.NoError(t, err)
	require.Len(t, generator.captured, 1, "cleanup UR should be created for clone rules when orphanDownstreamOnPolicyDelete=false")
	require.Len(t, generator.captured[0].Spec.RuleContext, 1)
	assert.True(t, generator.captured[0].Spec.RuleContext[0].DeleteDownstream)
	require.Len(t, generator.captured[0].Status.GeneratedResources, 1)
	assert.Equal(t, "ConfigMap", generator.captured[0].Status.GeneratedResources[0].Kind)
	assert.Equal(t, "target-cm", generator.captured[0].Status.GeneratedResources[0].Name)
}
