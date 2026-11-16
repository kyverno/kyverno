package admissionpolicy

import (
	"encoding/json"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const incrementReplicasPolicy = `{
	"apiVersion": "admissionregistration.k8s.io/v1beta1",
	"kind": "MutatingAdmissionPolicy",
	"metadata": {"name": "increment-replicas"},
	"spec": {
		"matchConstraints": {"resourceRules": [{"apiGroups": ["apps"], "apiVersions": ["v1"], "operations": ["CREATE"], "resources": ["deployments"]}]},
		"failurePolicy": "Fail",
		"mutations": [{"patchType": "ApplyConfiguration", "applyConfiguration": {"expression": "Object{ spec: Object.spec{ replicas: object.spec.replicas + 1 } }"}}]
	}
}`

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

func newBinding(name string) admissionregistrationv1beta1.MutatingAdmissionPolicyBinding {
	return admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{PolicyName: "increment-replicas"},
	}
}

func replicasOf(t *testing.T, obj unstructured.Unstructured) int64 {
	t.Helper()
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	assert.NilError(t, err)
	assert.Assert(t, found, "spec.replicas not found")
	return replicas
}

// Test_Mutate_BindingsCompose checks that each binding of a MutatingAdmissionPolicy is evaluated
// against the object produced by the previous binding, as the API server does, and that the
// response keeps a rule for every binding that was applied.
func Test_Mutate_BindingsCompose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		bindings     []string
		wantReplicas int64
	}{
		{name: "no bindings", bindings: nil, wantReplicas: 2},
		{name: "one binding", bindings: []string{"first"}, wantReplicas: 2},
		{name: "two bindings", bindings: []string{"first", "second"}, wantReplicas: 3},
		{name: "three bindings", bindings: []string{"first", "second", "third"}, wantReplicas: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
			assert.NilError(t, json.Unmarshal([]byte(incrementReplicasPolicy), &policy))
			data := engineapi.NewMutatingAdmissionPolicyData(&policy)
			for _, name := range tt.bindings {
				data.AddBinding(newBinding(name))
			}

			resource, err := kubeutils.BytesToUnstructured([]byte(oneReplicaDeployment))
			assert.NilError(t, err)
			gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

			resp, err := Mutate(data, *resource, resource.GroupVersionKind(), gvr, map[string]map[string]string{}, nil, nil, true, false)
			assert.NilError(t, err)

			assert.Equal(t, replicasOf(t, resp.PatchedResource), tt.wantReplicas)
			// the input resource must not be modified
			assert.Equal(t, replicasOf(t, *resource), int64(1))

			wantRules := len(tt.bindings)
			if wantRules == 0 {
				wantRules = 1
			}
			assert.Equal(t, len(resp.PolicyResponse.Rules), wantRules)
			for _, rule := range resp.PolicyResponse.Rules {
				assert.Equal(t, rule.Status(), engineapi.RuleStatusPass)
			}
			assert.Equal(t, resp.PolicyResponse.RulesAppliedCount(), wantRules)
		})
	}
}
