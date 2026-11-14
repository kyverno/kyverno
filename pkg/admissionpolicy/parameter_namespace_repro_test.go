package admissionpolicy

import (
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestInvestigationParameterNamespace(t *testing.T) {
	param := func(namespace, limit string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]interface{}{"name": "replica-limits", "namespace": namespace},
			"data":     map[string]interface{}{"maxReplicas": limit},
		}}
	}
	trusted := param("trusted", "5")
	foreign := param("foreign", "100")
	for _, tc := range []struct {
		name   string
		params []*unstructured.Unstructured
	}{
		{"trusted_only_control", []*unstructured.Unstructured{trusted}},
		{"trusted_first_control", []*unstructured.Unstructured{trusted, foreign}},
		{"foreign_first", []*unstructured.Unstructured{foreign, trusted}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					ParamKind:   &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
					Validations: []admissionregistrationv1.Validation{{Expression: "object.spec.replicas <= int(params.data.maxReplicas)"}},
				},
			}
			binding := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName:        policy.Name,
					ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
					ParamRef:          &admissionregistrationv1.ParamRef{Name: "replica-limits", Namespace: "trusted"},
				},
			}
			data := engineapi.NewValidatingAdmissionPolicyData(policy, binding)
			for _, p := range tc.params {
				data.AddParam(p.DeepCopy())
			}
			resource := unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]interface{}{"name": "workload", "namespace": "default"},
				"spec":     map[string]interface{}{"replicas": int64(10)},
			}}
			response, err := Validate(data, resource, resource.GroupVersionKind(), schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil, nil, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(response.PolicyResponse.Rules) != 1 {
				t.Fatalf("want one result, got %d", len(response.PolicyResponse.Rules))
			}
			status := response.PolicyResponse.Rules[0].Status()
			t.Logf("got %s; expected fail using trusted/replica-limits (limit 5)", status)
			if status != engineapi.RuleStatusFail {
				t.Fatalf("wrong namespace parameter selected: got %s, want fail", status)
			}
		})
	}
}

func TestInvestigationMAPParameterNamespace(t *testing.T) {
	policy := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1beta1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			Mutations: []admissionregistrationv1beta1.Mutation{{
				PatchType:          admissionregistrationv1beta1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1beta1.ApplyConfiguration{Expression: "Object{spec: Object.spec{replicas: int(params.data.maxReplicas)}}"},
			}},
		},
	}
	binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "replica-limit"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: policy.Name,
			ParamRef:   &admissionregistrationv1beta1.ParamRef{Name: "replica-limits", Namespace: "trusted"},
		},
	}
	for _, foreignFirst := range []bool{false, true} {
		name := "trusted_first_control"
		if foreignFirst {
			name = "foreign_first"
		}
		t.Run(name, func(t *testing.T) {
			data := engineapi.NewMutatingAdmissionPolicyData(policy, binding)
			params := []struct{ namespace, limit string }{{"trusted", "5"}, {"foreign", "100"}}
			if foreignFirst {
				params[0], params[1] = params[1], params[0]
			}
			for _, p := range params {
				data.AddParam(&unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "v1", "kind": "ConfigMap",
					"metadata": map[string]interface{}{"name": "replica-limits", "namespace": p.namespace},
					"data":     map[string]interface{}{"maxReplicas": p.limit},
				}})
			}
			resource := unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]interface{}{"name": "workload", "namespace": "default"},
				"spec":     map[string]interface{}{"replicas": int64(10)},
			}}
			response, err := Mutate(data, resource, resource.GroupVersionKind(), schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil, nil, nil, true, false)
			if err != nil {
				t.Fatal(err)
			}
			replicas, found, err := unstructured.NestedInt64(response.PatchedResource.Object, "spec", "replicas")
			if err != nil || !found {
				t.Fatalf("missing replicas: %v", err)
			}
			t.Logf("got replicas=%d; expected 5 from trusted/replica-limits", replicas)
			if replicas != 5 {
				t.Fatalf("wrong namespace parameter selected: got replicas=%d, want 5", replicas)
			}
		})
	}
}
