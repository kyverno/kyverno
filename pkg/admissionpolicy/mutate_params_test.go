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

// namespacedDisco makes the fake discovery report every resource as namespaced,
// so that CollectParams treats a ConfigMap paramKind as namespace-scoped.
type namespacedDisco struct {
	dclient.IDiscovery
}

func (d namespacedDisco) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	res, err := d.IDiscovery.FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	for k, v := range res {
		v.Namespaced = true
		res[k] = v
	}
	return res, nil
}

func Test_Mutate_ClusterScopedResourceWithParamRef(t *testing.T) {
	rawPolicy := []byte(`{
  "apiVersion": "admissionregistration.k8s.io/v1beta1",
  "kind": "MutatingAdmissionPolicy",
  "metadata": {"name": "label-namespaces-with-params"},
  "spec": {
    "paramKind": {"apiVersion": "v1", "kind": "ConfigMap"},
    "matchConstraints": {
      "resourceRules": [{
        "apiGroups": [""],
        "apiVersions": ["v1"],
        "operations": ["CREATE"],
        "resources": ["namespaces"]
      }]
    },
    "failurePolicy": "Fail",
    "reinvocationPolicy": "Never",
    "mutations": [{
      "patchType": "ApplyConfiguration",
      "applyConfiguration": {
        "expression": "object.metadata.?labels[\"bug3\"].hasValue() ? Object{} : Object{ metadata: Object.metadata{ labels: {\"bug3\": \"repro\"}}}"
      }
    }]
  }
}`)
	rawResource := []byte(`{
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {"name": "test-ns"}
}`)

	tests := []struct {
		name              string
		paramRefNamespace string
		wantErr           string
	}{
		{name: "paramRef.namespace set", paramRefNamespace: "default"},
		{name: "paramRef.namespace unset", paramRefNamespace: "", wantErr: "cluster-scoped"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
			assert.NilError(t, json.Unmarshal(rawPolicy, &policy))

			allow := admissionregistrationv1beta1.AllowAction
			binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding"},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
					PolicyName: policy.Name,
					ParamRef: &admissionregistrationv1beta1.ParamRef{
						Name:                    "map-params",
						Namespace:               tc.paramRefNamespace,
						ParameterNotFoundAction: &allow,
					},
				},
			}

			resource, err := kubeutils.BytesToUnstructured(rawResource)
			assert.NilError(t, err)
			gvk := resource.GroupVersionKind()
			gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}

			// param ConfigMap seeded into the fake dynamic client
			cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
			cm := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "map-params", "namespace": "default"},
				"data":       map[string]any{"key": "value"},
			}}
			scheme := runtime.NewScheme()
			scheme.AddKnownTypeWithName(cmGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				scheme,
				map[schema.GroupVersionResource]string{{Version: "v1", Resource: "configmaps"}: "ConfigMapList"},
				cm,
			)
			kube := kubefake.NewSimpleClientset(
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}},
			)
			client := dclient.NewFakeClientWithDisco(dyn, kube, namespacedDisco{dclient.NewFakeDiscoveryClient(nil)})

			data := engineapi.NewMutatingAdmissionPolicyData(&policy, binding)
			er, err := Mutate(data, *resource, gvk, gvr, nil, client, nil, false, true)

			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, er.PatchedResource.GetLabels()["bug3"], "repro")
		})
	}
}
