package mutation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mockClient struct {
	engineapi.Client
	resources []engineapi.Resource
}

func (m *mockClient) GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string, lselector *metav1.LabelSelector) ([]engineapi.Resource, error) {
	var matched []engineapi.Resource
	for _, r := range m.resources {
		if (name == "" || r.Unstructured.GetName() == name) && (namespace == "" || r.Unstructured.GetNamespace() == namespace) {
			matched = append(matched, r)
		}
	}
	return matched, nil
}

func Test_mutateExistingHandler_Process_ContextIsolation(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)

	target1JSON := `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "target-one",
			"namespace": "default"
		},
		"data": {
			"key": "val1"
		}
	}`

	target2JSON := `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "target-two",
			"namespace": "default"
		},
		"data": {
			"key": "val2"
		}
	}`

	t1, err := kubeutils.BytesToUnstructured([]byte(target1JSON))
	require.NoError(t, err)
	t2, err := kubeutils.BytesToUnstructured([]byte(target2JSON))
	require.NoError(t, err)

	client := &mockClient{
		resources: []engineapi.Resource{
			{Group: "", Version: "v1", Resource: "configmaps", Unstructured: *t1},
			{Group: "", Version: "v1", Resource: "configmaps", Unstructured: *t2},
		},
	}

	triggerJSON := `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "trigger-cm",
			"namespace": "default"
		}
	}`
	trigger, err := kubeutils.BytesToUnstructured([]byte(triggerJSON))
	require.NoError(t, err)

	policyJSON := `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "test-context-isolation"
		},
		"spec": {
			"rules": [
				{
					"name": "mutate-targets",
					"match": {
						"any": [
							{
								"resources": {
									"kinds": ["ConfigMap"],
									"names": ["trigger-cm"]
								}
							}
						]
					},
					"mutate": {
						"targets": [
							{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"name": "target-one",
								"namespace": "default",
								"context": [
									{
										"name": "injectedVar",
										"variable": {
											"value": "secret-from-target-0"
										}
									}
								]
							},
							{
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"name": "target-two",
								"namespace": "default"
							}
						],
						"patchStrategicMerge": {
							"data": {
								"token": "{{ injectedVar || 'isolated-default' }}"
							}
						}
					}
				}
			]
		}
	}`

	var cpol kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(policyJSON), &cpol)
	require.NoError(t, err)

	pCtx, err := policycontext.NewPolicyContext(jp, *trigger, kyvernov1.Create, nil, cfg)
	require.NoError(t, err)
	pCtx = pCtx.WithPolicy(&cpol).WithNewResource(*trigger)

	handler, err := NewMutateExistingHandler(client, false, cfg)
	require.NoError(t, err)

	rule := cpol.GetSpec().Rules[0]
	contextLoader := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		for _, entry := range contextEntries {
			if entry.Variable != nil {
				raw, err := json.Marshal(entry.Variable.Value)
				if err != nil {
					return err
				}
				if err := jsonContext.AddContextEntry(entry.Name, raw); err != nil {
					return err
				}
			}
		}
		return nil
	}

	_, responses := handler.Process(context.TODO(), logr.Discard(), pCtx, *trigger, rule, contextLoader, nil)
	require.Equal(t, 2, len(responses))
	require.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
	require.Equal(t, engineapi.RuleStatusPass, responses[1].Status())

	patched0, _, _ := responses[0].PatchedTarget()
	require.NotNil(t, patched0)
	data0, found, err := unstructured.NestedStringMap(patched0.UnstructuredContent(), "data")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "secret-from-target-0", data0["token"])

	patched1, _, _ := responses[1].PatchedTarget()
	require.NotNil(t, patched1)
	data1, found, err := unstructured.NestedStringMap(patched1.UnstructuredContent(), "data")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "isolated-default", data1["token"])

	_, err = pCtx.JSONContext().Query("injectedVar")
	require.ErrorContains(t, err, `Unknown key "injectedVar" in path`)
	_, err = pCtx.JSONContext().Query("target")
	require.ErrorContains(t, err, `Unknown key "target" in path`)
	value, err := pCtx.JSONContext().Query("request.object.metadata.name")
	require.NoError(t, err)
	require.Equal(t, "trigger-cm", value)
}
