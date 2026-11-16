package admissionpolicy

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const oneReplicaDeployment = `{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {"name": "nginx", "namespace": "default"},
	"spec": {
		"replicas": 1,
		"selector": {"matchLabels": {"app": "nginx"}},
		"template": {
			"metadata": {"labels": {"app": "nginx"}},
			"spec": {"containers": [{"name": "nginx", "image": "nginx"}]}
		}
	}
}`

var deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

// incrementReplicasPolicy returns a policy that adds 1 to spec.replicas, guarded by the given
// match condition expressions.
func incrementReplicasPolicy(t *testing.T, matchConditions ...string) *admissionregistrationv1beta1.MutatingAdmissionPolicy {
	t.Helper()
	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal([]byte(`{
		"apiVersion": "admissionregistration.k8s.io/v1beta1",
		"kind": "MutatingAdmissionPolicy",
		"metadata": {"name": "increment-replicas"},
		"spec": {
			"matchConstraints": {"resourceRules": [{"apiGroups": ["apps"], "apiVersions": ["v1"], "operations": ["CREATE"], "resources": ["deployments"]}]},
			"failurePolicy": "Fail",
			"mutations": [{"patchType": "ApplyConfiguration", "applyConfiguration": {"expression": "Object{ spec: Object.spec{ replicas: object.spec.replicas + 1 } }"}}]
		}
	}`), &policy))
	for i, expression := range matchConditions {
		policy.Spec.MatchConditions = append(policy.Spec.MatchConditions, admissionregistrationv1beta1.MatchCondition{
			Name:       "condition-" + string(rune('a'+i)),
			Expression: expression,
		})
	}
	return &policy
}

func newBinding(name string) admissionregistrationv1beta1.MutatingAdmissionPolicyBinding {
	return admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{PolicyName: "increment-replicas"},
	}
}

// newFakeClient returns a client that knows the "default" namespace, so that Mutate takes the
// client-backed path used by the admission controller and background scans.
func newFakeClient() dclient.Interface {
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	return dclient.NewFakeClientWithDisco(dyn, kube, dclient.NewFakeDiscoveryClient(nil))
}

func replicasOf(t *testing.T, obj unstructured.Unstructured) int64 {
	t.Helper()
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	assert.NilError(t, err)
	assert.Assert(t, found, "spec.replicas not found")
	return replicas
}

// Test_Mutate_BindingsCompose checks that each binding of a MutatingAdmissionPolicy is evaluated
// against the object produced by the bindings before it, as the API server does, that the response
// keeps a rule for every binding, and that the response's Resource stays the original object so the
// patches cover every binding. Each case runs with and without a client, which use different loops.
func Test_Mutate_BindingsCompose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		matchConditions []string
		bindings        []string
		wantReplicas    int64
		wantStatuses    []engineapi.RuleStatus
	}{
		{
			name:         "no bindings",
			wantReplicas: 2,
			wantStatuses: []engineapi.RuleStatus{engineapi.RuleStatusPass},
		},
		{
			name:         "one binding",
			bindings:     []string{"first"},
			wantReplicas: 2,
			wantStatuses: []engineapi.RuleStatus{engineapi.RuleStatusPass},
		},
		{
			name:         "two bindings",
			bindings:     []string{"first", "second"},
			wantReplicas: 3,
			wantStatuses: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusPass},
		},
		{
			name:         "three bindings",
			bindings:     []string{"first", "second", "third"},
			wantReplicas: 4,
			wantStatuses: []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusPass, engineapi.RuleStatusPass},
		},
		{
			// the second binding sees replicas: 2, so its match condition is false and it is skipped
			name:            "second binding skipped by match condition",
			matchConditions: []string{"object.spec.replicas < 2"},
			bindings:        []string{"first", "second"},
			wantReplicas:    2,
			wantStatuses:    []engineapi.RuleStatus{engineapi.RuleStatusPass, engineapi.RuleStatusSkip},
		},
		{
			// the second binding sees replicas: 2, so its match condition reads a missing field and fails;
			// the first binding's mutation and rule must be kept
			name:            "second binding fails to evaluate its match condition",
			matchConditions: []string{"object.spec.replicas == 1 || object.spec.missing == 1"},
			bindings:        []string{"first", "second"},
			wantReplicas:    2,
			wantStatuses:    []engineapi.RuleStatus{engineapi.RuleStatusPass},
		},
	}
	for _, tt := range tests {
		for _, withClient := range []bool{false, true} {
			name := tt.name + "/without client"
			if withClient {
				name = tt.name + "/with client"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				data := engineapi.NewMutatingAdmissionPolicyData(incrementReplicasPolicy(t, tt.matchConditions...))
				for _, binding := range tt.bindings {
					data.AddBinding(newBinding(binding))
				}
				resource, err := kubeutils.BytesToUnstructured([]byte(oneReplicaDeployment))
				assert.NilError(t, err)

				var client dclient.Interface
				if withClient {
					client = newFakeClient()
				}
				resp, err := Mutate(data, *resource, resource.GroupVersionKind(), deploymentsGVR, map[string]map[string]string{}, client, nil, !withClient, false)
				assert.NilError(t, err)

				assert.Equal(t, replicasOf(t, resp.PatchedResource), tt.wantReplicas)
				// the response keeps the original resource, and the input is not modified
				assert.Equal(t, replicasOf(t, resp.Resource), int64(1))
				assert.Equal(t, replicasOf(t, *resource), int64(1))

				var statuses []engineapi.RuleStatus
				var applied int
				for _, rule := range resp.PolicyResponse.Rules {
					statuses = append(statuses, rule.Status())
					if rule.Status() == engineapi.RuleStatusPass {
						applied++
					}
				}
				assert.DeepEqual(t, statuses, tt.wantStatuses)
				assert.Equal(t, resp.PolicyResponse.RulesAppliedCount(), applied)
			})
		}
	}
}

// namespacedDiscovery reports every resource as namespaced: the shared fake discovery client returns
// empty API resources, which makes namespaced kinds such as ConfigMaps look cluster-scoped.
type namespacedDiscovery struct {
	dclient.IDiscovery
}

func (d namespacedDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	resources, err := d.IDiscovery.FindResources(group, version, kind, subresource)
	for description, resource := range resources {
		resource.Namespaced = true
		resources[description] = resource
	}
	return resources, err
}

func incrementParam(name, increment string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"data":       map[string]any{"increment": increment},
	}}
}

// Test_Mutate_ParameterizedBindingsCompose checks composition through the parameterized branches:
// each binding selects its own param by name, and the second binding sees the first one's result.
func Test_Mutate_ParameterizedBindingsCompose(t *testing.T) {
	t.Parallel()
	for _, withClient := range []bool{false, true} {
		name := "without client"
		if withClient {
			name = "with client"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			policy := incrementReplicasPolicy(t)
			policy.Spec.ParamKind = &admissionregistrationv1beta1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
			policy.Spec.Mutations[0].ApplyConfiguration.Expression = "Object{ spec: Object.spec{ replicas: object.spec.replicas + int(params.data.increment) } }"

			params := []*unstructured.Unstructured{incrementParam("inc-1", "1"), incrementParam("inc-2", "2")}
			data := engineapi.NewMutatingAdmissionPolicyData(policy)
			for _, param := range params {
				binding := newBinding("binding-" + param.GetName())
				binding.Spec.ParamRef = &admissionregistrationv1beta1.ParamRef{Name: param.GetName(), Namespace: "default"}
				data.AddBinding(binding)
				data.AddParam(param)
			}

			resource, err := kubeutils.BytesToUnstructured([]byte(oneReplicaDeployment))
			assert.NilError(t, err)

			var client dclient.Interface
			if withClient {
				kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
				dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
					runtime.NewScheme(),
					map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"},
					params[0], params[1],
				)
				client = dclient.NewFakeClientWithDisco(dyn, kube, namespacedDiscovery{dclient.NewFakeDiscoveryClient(nil)})
			}
			resp, err := Mutate(data, *resource, resource.GroupVersionKind(), deploymentsGVR, map[string]map[string]string{}, client, nil, !withClient, false)
			assert.NilError(t, err)

			// 1 + 1 (inc-1) + 2 (inc-2)
			assert.Equal(t, replicasOf(t, resp.PatchedResource), int64(4))
			assert.Equal(t, replicasOf(t, resp.Resource), int64(1))
			assert.Equal(t, len(resp.PolicyResponse.Rules), 2)
			for _, rule := range resp.PolicyResponse.Rules {
				assert.Equal(t, rule.Status(), engineapi.RuleStatusPass)
			}
			assert.Equal(t, resp.PolicyResponse.RulesAppliedCount(), 2)
		})
	}
}
