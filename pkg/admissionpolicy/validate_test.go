package admissionpolicy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

// decodeVAP decodes a single ValidatingAdmissionPolicy YAML fixture; every fixture in this
// file's tables is one such document.
func decodeVAP(t *testing.T, raw []byte) *admissionregistrationv1.ValidatingAdmissionPolicy {
	t.Helper()
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	assert.NilError(t, yaml.Unmarshal(raw, &policy))
	return &policy
}

func TestGetKinds(t *testing.T) {
	type test struct {
		name      string
		policy    []byte
		wantKinds []string
	}

	tests := []test{
		{
			name: "Matching pods",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-1"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["pods"]
  validations:
    - expression: "object.metadata.name.matches('nginx')"
`),
			wantKinds: []string{"v1/Pod"},
		},
		{
			name: "Matching subresource",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-1"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["pods/log"]
  validations:
    - expression: "object.metadata.name.matches('nginx')"
`),
			wantKinds: []string{"v1/Pod/log"},
		},
		{
			name: "Matching deployments, replicasets, daemonsets and statefulsets",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-2"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments", "replicasets", "daemonsets", "statefulsets"]
  validations:
    - expression: "object.spec.replicas <= 5"
`),
			wantKinds: []string{"apps/v1/Deployment", "apps/v1/ReplicaSet", "apps/v1/DaemonSet", "apps/v1/StatefulSet"},
		},
		{
			name: "Matching deployments/scale",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-3"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments/scale"]
  validations:
    - expression: "object.spec.replicas <= 5"
`),
			wantKinds: []string{"apps/v1/Deployment/scale"},
		},
		{
			name: "Matching jobs and cronjobs",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-4"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   ["batch"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["jobs", "cronjobs"]
  validations:
    - expression: "object.spec.jobTemplate.spec.template.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.readOnlyRootFilesystem) &&  container.securityContext.readOnlyRootFilesystem == true)"
`),
			wantKinds: []string{"batch/v1/Job", "batch/v1/CronJob"},
		},
		{
			name: "Multiple resource rules",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-5"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["pods"]
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments", "replicasets", "daemonsets", "statefulsets"]
      - apiGroups:   ["batch"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["jobs", "cronjobs"]
  validations:
    - expression: "object.spec.replicas <= 5"
`),
			wantKinds: []string{"v1/Pod", "apps/v1/Deployment", "apps/v1/ReplicaSet", "apps/v1/DaemonSet", "apps/v1/StatefulSet", "batch/v1/Job", "batch/v1/CronJob"},
		},
		{
			name: "Match subresource",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: match-pod-exec
spec:
  failurePolicy: Fail
  matchConditions:
  - expression: request.operation == 'CONNECT'
    name: operation-should-be-connect
  matchConstraints:
    resourceRules:
    - apiGroups:
      - ""
      apiVersions:
      - v1
      operations:
      - CREATE
      - UPDATE
      - CONNECT
      resources:
      - pods/exec
  validations:
  - expression: request.namespace != 'pci'
    message: Pods in this namespace may not be exec'd into.
`),
			wantKinds: []string{"v1/Pod/exec"},
		},
		{
			name: "skip incomplete resource rules",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-5"
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   []
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["pods"]
      - apiGroups:   ["apps"]
        apiVersions: ["v1"]
        operations:  ["CREATE", "UPDATE"]
        resources:   ["deployments", "replicasets", "daemonsets", "statefulsets"]
      - apiGroups:   ["batch"]
        apiVersions: []
        operations:  ["CREATE", "UPDATE"]
        resources:   ["jobs", "cronjobs"]
  validations:
    - expression: "object.spec.replicas <= 5"
`),
			wantKinds: []string{"apps/v1/Deployment", "apps/v1/ReplicaSet", "apps/v1/DaemonSet", "apps/v1/StatefulSet"},
		},
		{
			name: "No matchConstraints",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: "policy-5"
spec:
  failurePolicy: Fail
  validations:
    - expression: "object.spec.replicas <= 5"
`),
			wantKinds: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := decodeVAP(t, tt.policy)
			restMapper, err := utils.GetRESTMapper(nil)
			assert.NilError(t, err)
			kinds := GetKinds(policy.Spec.MatchConstraints, restMapper)
			if !reflect.DeepEqual(kinds, tt.wantKinds) {
				t.Errorf("Expected %v, got %v", tt.wantKinds, kinds)
			}
		})
	}
}

func Test_ValidateResourceNilMatchConstraints(t *testing.T) {
	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "test-pod",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "name": "nginx",
                "image": "nginx:latest"
            }
        ]
    }
}`)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vap-nil-matchconstraints",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			// MatchConstraints is omitted, so it's nil by default
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}

	gvk := resource.GroupVersionKind()
	restMapper, err := utils.GetRESTMapper(nil)
	assert.NilError(t, err)

	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	assert.NilError(t, err)

	gvr := mapping.Resource
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: resource.GetNamespace(),
		},
	}

	response, err := validateResource(policy, nil, *resource, nil, ns, a)
	assert.NilError(t, err)
	assert.Equal(t, len(response.PolicyResponse.Rules), 1)
}

// TestGetGVRs_SkipsUnsafeResourceSegments covers GHSA-4pqm-wqq6-h9q2 for the
// deleting controller: GetGVRs builds a GVR straight from the policy strings
// with no mapper lookup, and the dynamic client joins that resource into the
// request path with path.Join, so a ".." entry would turn the controller's
// LIST into an attacker-chosen path under its own ServiceAccount.
func TestGetGVRs_SkipsUnsafeResourceSegments(t *testing.T) {
	rules := func(group, version string, resources ...string) *admissionregistrationv1.MatchResources {
		return &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{group},
						APIVersions: []string{version},
						Resources:   resources,
					},
				},
			}},
		}
	}

	got := GetGVRs(rules("", "v1", "pods/../namespaces/kube-system/secrets", "..", ".", "configmaps"), nil)
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Resource, "configmaps")

	assert.Equal(t, len(GetGVRs(rules("..", "v1", "pods"), nil)), 0, "unsafe api group")
	assert.Equal(t, len(GetGVRs(rules("", "..", "pods"), nil)), 0, "unsafe api version")

	// only the core group may be empty: path.Join would collapse an empty version or
	// resource and address a different API path (e.g. /api/v1 for an empty resource)
	assert.Equal(t, len(GetGVRs(rules("", "", "pods"), nil)), 0, "empty api version")
	assert.Equal(t, len(GetGVRs(rules("", "v1", ""), nil)), 0, "empty resource")
	assert.Equal(t, len(GetGVRs(rules("apps", "v1", ""), nil)), 0, "empty resource, named group")

	// wildcards and the core group must still resolve
	wild := GetGVRs(rules("", "v1", "*"), nil)
	assert.Equal(t, len(wild), 1)
	assert.Equal(t, wild[0].Resource, "*")
}

func Test_ProcessVAPWithoutClient_MatchConstraintsEvaluation(t *testing.T) {
	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: match-only-services
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["services"]
  validations:
    - expression: "object.spec.type == 'ClusterIP'"
`)
	policy := decodeVAP(t, rawPolicy)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "my-config",
        "namespace": "default"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-1"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "match-only-services",
			},
		},
	}

	er, err := processVAPWithoutClient(policy, bindings, *resource, ns, nil, a, "default/ConfigMap/my-config")
	assert.NilError(t, err)
	assert.Equal(t, len(er.PolicyResponse.Rules), 0)
}

func Test_ProcessVAP_MultiBindingAggregation(t *testing.T) {
	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: multi-binding-policy
spec:
  failurePolicy: Fail
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["configmaps"]
  validations:
    - expression: "object.metadata.name == 'allowed-cm'"
`)
	policy := decodeVAP(t, rawPolicy)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "disallowed-cm",
        "namespace": "default"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-1"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "multi-binding-policy",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-2"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "multi-binding-policy",
			},
		},
	}

	er, err := processVAPWithoutClient(policy, bindings, *resource, ns, nil, a, "default/ConfigMap/disallowed-cm")
	assert.NilError(t, err)
	assert.Equal(t, len(er.PolicyResponse.Rules), 2)
	assert.Equal(t, er.PolicyResponse.Rules[0].Name(), "multi-binding-policy")
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusFail)
	assert.Assert(t, er.PolicyResponse.Rules[0].ValidatingAdmissionPolicyBinding() != nil)
	assert.Equal(t, er.PolicyResponse.Rules[0].ValidatingAdmissionPolicyBinding().GetName(), "binding-1")

	assert.Equal(t, er.PolicyResponse.Rules[1].Name(), "multi-binding-policy")
	assert.Equal(t, er.PolicyResponse.Rules[1].Status(), engineapi.RuleStatusFail)
	assert.Assert(t, er.PolicyResponse.Rules[1].ValidatingAdmissionPolicyBinding() != nil)
	assert.Equal(t, er.PolicyResponse.Rules[1].ValidatingAdmissionPolicyBinding().GetName(), "binding-2")
	assert.Equal(t, er.IsSuccessful(), false)
	assert.Equal(t, er.IsFailed(), true)
}

func Test_ProcessVAPWithoutClient_MultipleParamsWithoutSelector_FailsAdmission(t *testing.T) {
	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: param-policy
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["configmaps"]
  validations:
    - expression: "params != null"
`)
	policy := decodeVAP(t, rawPolicy)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "target-cm",
        "namespace": "default"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-named"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "param-policy",
				ParamRef: &admissionregistrationv1.ParamRef{
					Name:      "named-cm",
					Namespace: "default",
				},
			},
		},
	}

	cm1, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"named-cm","namespace":"default"}}`))
	assert.NilError(t, err)
	cm2, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"named-cm","namespace":"default"}}`))
	assert.NilError(t, err)

	// In offline evaluation, when multiple parameter objects match a single-param binding, an admission error must be returned.
	er, err := processVAPWithoutClient(policy, bindings, *resource, ns, []runtime.Object{cm1, cm2}, a, "default/ConfigMap/target-cm")
	assert.NilError(t, err)
	assert.Equal(t, er.IsSuccessful(), false)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "multiple parameter resources match paramRef without selector"))
}

func Test_ProcessVAPWithoutClient_ClusterScopedTarget_NamespacedParamWithoutNamespace_Fails(t *testing.T) {
	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: cluster-target-vap-policy
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["namespaces"]
  validations:
    - expression: "params != null"
`)
	policy := decodeVAP(t, rawPolicy)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "Namespace",
    "metadata": {
        "name": "target-ns"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, "", resource.GetName(), gvr, "", admission.Create, nil, false, nil)

	bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-cluster-target"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: "cluster-target-vap-policy",
				ParamRef: &admissionregistrationv1.ParamRef{
					Name: "config-a",
				},
			},
		},
	}

	cm, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"config-a","namespace":"tenant-1"}}`))
	assert.NilError(t, err)

	er, err := processVAPWithoutClient(policy, bindings, *resource, nil, []runtime.Object{cm}, a, "target-ns")
	assert.NilError(t, err)
	assert.Equal(t, er.IsSuccessful(), false)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "can't use namespaced paramRef to match cluster-scoped resources"))
}

func Test_ProcessVAPWithoutClient_FiltersParamsByParamKind(t *testing.T) {
	t.Run("cluster target preflight: apiVersion filtering independent of kind", func(t *testing.T) {
		rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: cluster-filter-policy
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["namespaces"]
  validations:
    - expression: "params != null"
`)
		policy := decodeVAP(t, rawPolicy)
		rawResource := []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"target-ns"}}`)
		resource, err := kubeutils.BytesToUnstructured(rawResource)
		assert.NilError(t, err)

		gvk := resource.GroupVersionKind()
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
		a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, "", resource.GetName(), gvr, "", admission.Create, nil, false, nil)

		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-cluster"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "cluster-filter-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name: "cluster-cm",
					},
				},
			},
		}

		// cmWrongVersion has same Kind ("ConfigMap"), but different apiVersion.
		cmWrongVersion, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"custom.io/v1beta1","kind":"ConfigMap","metadata":{"name":"cluster-cm","namespace":"tenant-1"}}`))
		assert.NilError(t, err)
		cmRightVersion, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cluster-cm"}}`))
		assert.NilError(t, err)

		// Case A: Wrong version present alongside valid cluster param does not fail preflight
		er, err := processVAPWithoutClient(policy, bindings, *resource, nil, []runtime.Object{cmWrongVersion, cmRightVersion}, a, "target-ns")
		assert.NilError(t, err)
		assert.Equal(t, er.IsSuccessful(), true)
		assert.Equal(t, len(er.PolicyResponse.Rules), 1)
		assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

		// Case B: Wrong version only cannot supply params
		erWrong, err := processVAPWithoutClient(policy, bindings, *resource, nil, []runtime.Object{cmWrongVersion}, a, "target-ns")
		assert.NilError(t, err)
		assert.Equal(t, erWrong.IsSuccessful(), false)
		assert.Equal(t, len(erWrong.PolicyResponse.Rules), 1)
		assert.Equal(t, erWrong.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
	})

	t.Run("parameter selection: apiVersion filtering independent of kind", func(t *testing.T) {
		rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: param-filter-policy
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["configmaps"]
  validations:
    - expression: "params != null"
`)
		policy := decodeVAP(t, rawPolicy)
		rawResource := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"target-cm","namespace":"default"}}`)
		resource, err := kubeutils.BytesToUnstructured(rawResource)
		assert.NilError(t, err)

		gvk := resource.GroupVersionKind()
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-named"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "param-filter-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:      "shared-name",
						Namespace: "default",
					},
				},
			},
		}

		// cmWrongVersion has same Kind ("ConfigMap"), same name, and same namespace, but different apiVersion.
		cmWrongVersion, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"custom.io/v1beta1","kind":"ConfigMap","metadata":{"name":"shared-name","namespace":"default"}}`))
		assert.NilError(t, err)
		cmRightVersion, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"shared-name","namespace":"default"}}`))
		assert.NilError(t, err)

		// Case A: Disambiguates identical kind, name, and namespace by apiVersion without ambiguous match error
		er, err := processVAPWithoutClient(policy, bindings, *resource, nil, []runtime.Object{cmWrongVersion, cmRightVersion}, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, er.IsSuccessful(), true)
		assert.Equal(t, len(er.PolicyResponse.Rules), 1)
		assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

		// Case B: Wrong version only cannot match paramRef
		erWrong, err := processVAPWithoutClient(policy, bindings, *resource, nil, []runtime.Object{cmWrongVersion}, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, erWrong.IsSuccessful(), false)
		assert.Equal(t, len(erWrong.PolicyResponse.Rules), 1)
		assert.Equal(t, erWrong.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
	})
}

func Test_ProcessVAP_FailurePolicy_Ignore_SuppressesParamResolutionErrors(t *testing.T) {
	failPolicy := admissionregistrationv1.Fail
	ignorePolicy := admissionregistrationv1.Ignore
	denyAction := admissionregistrationv1.DenyAction

	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: vap-failure-policy
spec:
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["configmaps"]
  validations:
    - expression: "params != null && params.data.allowed == 'true'"
`)

	allowedCM, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"allowed-cm","namespace":"default"},"data":{"allowed":"true"}}`))
	assert.NilError(t, err)

	ambiguousCM1, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"ambiguous-cm","namespace":"default","uid":"cm-1"}}`))
	assert.NilError(t, err)
	ambiguousCM2, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"ambiguous-cm","namespace":"default","uid":"cm-2"}}`))
	assert.NilError(t, err)

	validBinding := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "binding-allowed"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "vap-failure-policy",
			ParamRef: &admissionregistrationv1.ParamRef{
				Name:      "allowed-cm",
				Namespace: "default",
			},
		},
	}

	testCases := []struct {
		name            string
		failurePolicy   *admissionregistrationv1.FailurePolicyType
		faultBinding    admissionregistrationv1.ValidatingAdmissionPolicyBinding
		paramPool       []runtime.Object
		targetIsCluster bool
	}{
		{
			name:          "missing named parameter with DenyAction - failurePolicy nil (defaults to Fail)",
			failurePolicy: nil,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-missing-deny"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:                    "missing-cm",
						Namespace:               "default",
						ParameterNotFoundAction: &denyAction,
					},
				},
			},
			paramPool: []runtime.Object{allowedCM},
		},
		{
			name:          "missing named parameter with DenyAction - failurePolicy Fail",
			failurePolicy: &failPolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-missing-deny"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:                    "missing-cm",
						Namespace:               "default",
						ParameterNotFoundAction: &denyAction,
					},
				},
			},
			paramPool: []runtime.Object{allowedCM},
		},
		{
			name:          "missing named parameter with DenyAction - failurePolicy Ignore",
			failurePolicy: &ignorePolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-missing-deny"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:                    "missing-cm",
						Namespace:               "default",
						ParameterNotFoundAction: &denyAction,
					},
				},
			},
			paramPool: []runtime.Object{allowedCM},
		},
		{
			name:          "ambiguous param matches without selector - failurePolicy Fail",
			failurePolicy: &failPolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-ambiguous"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name: "ambiguous-cm",
					},
				},
			},
			paramPool: []runtime.Object{ambiguousCM1, ambiguousCM2, allowedCM},
		},
		{
			name:          "ambiguous param matches without selector - failurePolicy Ignore",
			failurePolicy: &ignorePolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-ambiguous"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name: "ambiguous-cm",
					},
				},
			},
			paramPool: []runtime.Object{ambiguousCM1, ambiguousCM2, allowedCM},
		},
		{
			name:          "cluster target with namespaced paramRef - failurePolicy Fail",
			failurePolicy: &failPolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-cluster-target"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name: "allowed-cm",
					},
				},
			},
			paramPool:       []runtime.Object{allowedCM},
			targetIsCluster: true,
		},
		{
			name:          "cluster target with namespaced paramRef - failurePolicy Ignore",
			failurePolicy: &ignorePolicy,
			faultBinding: admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-cluster-target"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "vap-failure-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name: "allowed-cm",
					},
				},
			},
			paramPool:       []runtime.Object{allowedCM},
			targetIsCluster: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy := decodeVAP(t, rawPolicy)
			policy.Spec.FailurePolicy = tc.failurePolicy

			var rawResource []byte
			if tc.targetIsCluster {
				rawResource = []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"target-cm"}}`)
			} else {
				rawResource = []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"target-cm","namespace":"default"}}`)
			}
			resource, err := kubeutils.BytesToUnstructured(rawResource)
			assert.NilError(t, err)

			gvk := resource.GroupVersionKind()
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)

			bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				tc.faultBinding,
				validBinding,
			}

			er, err := processVAPWithoutClient(policy, bindings, *resource, nil, tc.paramPool, a, "default/ConfigMap/target-cm")
			assert.NilError(t, err)

			isFail := tc.failurePolicy == nil || *tc.failurePolicy == admissionregistrationv1.Fail
			if isFail {
				assert.Equal(t, er.IsSuccessful(), false)
				assert.Equal(t, len(er.PolicyResponse.Rules), 2)
				assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
				assert.Equal(t, er.PolicyResponse.Rules[0].ValidatingAdmissionPolicyBinding().GetName(), tc.faultBinding.GetName())
				if strings.Contains(tc.name, "ambiguous") {
					assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "multiple parameter resources match paramRef without selector"))
				}
				assert.Equal(t, er.PolicyResponse.Rules[1].Status(), engineapi.RuleStatusPass)
				assert.Equal(t, er.PolicyResponse.Rules[1].ValidatingAdmissionPolicyBinding().GetName(), validBinding.GetName())
			} else {
				// failurePolicy: Ignore suppresses faultBinding error, allows validBinding to evaluate and succeed
				assert.Equal(t, er.IsSuccessful(), true)
				assert.Equal(t, len(er.PolicyResponse.Rules), 1)
				assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
				assert.Equal(t, er.PolicyResponse.Rules[0].ValidatingAdmissionPolicyBinding().GetName(), validBinding.GetName())
			}
		})
	}
}

type testNamespacedDiscovery struct {
	dclient.IDiscovery
	namespaced bool
}

func (d *testNamespacedDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	res, err := d.IDiscovery.FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	out := make(map[dclient.TopLevelApiDescription]metav1.APIResource, len(res))
	for k, v := range res {
		v.Namespaced = d.namespaced
		out[k] = v
	}
	return out, nil
}

type testClientWithKube struct {
	dclient.Interface
	kubeClient kubernetes.Interface
}

func (c *testClientWithKube) GetKubeClient() kubernetes.Interface {
	if c.kubeClient != nil {
		return c.kubeClient
	}
	return c.Interface.GetKubeClient()
}

func Test_ProcessVAPWithClient_Scenarios(t *testing.T) {
	rawPolicy := []byte(`
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: client-test-policy
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups:   [""]
        apiVersions: ["v1"]
        operations:  ["CREATE"]
        resources:   ["configmaps"]
  validations:
    - expression: "params != null && params.data.allowed == 'true'"
`)
	policy := decodeVAP(t, rawPolicy)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "target-cm",
        "namespace": "default"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}

	t.Run("parameter not found with allow action skips binding", func(t *testing.T) {
		allowAction := admissionregistrationv1.AllowAction
		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-allow"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "client-test-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:                    "missing-cm",
						Namespace:               "default",
						ParameterNotFoundAction: &allowAction,
					},
				},
			},
		}

		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		baseClient, err := dclient.NewFakeClient(scheme, gvrToListKind, ns)
		assert.NilError(t, err)
		baseClient.SetDiscovery(&testNamespacedDiscovery{
			IDiscovery: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
				{Group: "", Version: "v1", Resource: "namespaces"},
				{Group: "", Version: "v1", Resource: "configmaps"},
			}),
			namespaced: true,
		})
		client := &testClientWithKube{
			Interface:  baseClient,
			kubeClient: kubefake.NewSimpleClientset(ns),
		}

		er, err := processVAPWithClient(policy, bindings, *resource, "default", ns, client, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, len(er.PolicyResponse.Rules), 0)
	})

	t.Run("parameter not found with deny action fails admission", func(t *testing.T) {
		denyAction := admissionregistrationv1.DenyAction
		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-deny-named"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "client-test-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:                    "missing-cm",
						Namespace:               "default",
						ParameterNotFoundAction: &denyAction,
					},
				},
			},
		}

		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		baseClient, err := dclient.NewFakeClient(scheme, gvrToListKind, ns)
		assert.NilError(t, err)
		baseClient.SetDiscovery(&testNamespacedDiscovery{
			IDiscovery: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
				{Group: "", Version: "v1", Resource: "namespaces"},
				{Group: "", Version: "v1", Resource: "configmaps"},
			}),
			namespaced: true,
		})
		client := &testClientWithKube{
			Interface:  baseClient,
			kubeClient: kubefake.NewSimpleClientset(ns),
		}

		er, err := processVAPWithClient(policy, bindings, *resource, "default", ns, client, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, er.IsSuccessful(), false)
		assert.Equal(t, len(er.PolicyResponse.Rules), 1)
		assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
		assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "missing-cm"))
	})

	t.Run("selector matching zero params with deny action fails admission", func(t *testing.T) {
		denyAction := admissionregistrationv1.DenyAction
		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-deny-selector"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "client-test-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Namespace: "default",
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nonexistent": "label",
							},
						},
						ParameterNotFoundAction: &denyAction,
					},
				},
			},
		}

		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		baseClient, err := dclient.NewFakeClient(scheme, gvrToListKind, ns)
		assert.NilError(t, err)
		baseClient.SetDiscovery(&testNamespacedDiscovery{
			IDiscovery: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
				{Group: "", Version: "v1", Resource: "namespaces"},
				{Group: "", Version: "v1", Resource: "configmaps"},
			}),
			namespaced: true,
		})
		client := &testClientWithKube{
			Interface:  baseClient,
			kubeClient: kubefake.NewSimpleClientset(ns),
		}

		er, err := processVAPWithClient(policy, bindings, *resource, "default", ns, client, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, er.IsSuccessful(), false)
		assert.Equal(t, len(er.PolicyResponse.Rules), 1)
		assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
		assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "no params found"))
	})

	t.Run("multi-binding aggregation with matched params passes admission", func(t *testing.T) {
		paramCM := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allowed-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"allowed": "true",
			},
		}

		bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-1"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "client-test-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:      "allowed-cm",
						Namespace: "default",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "binding-2"},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "client-test-policy",
					ParamRef: &admissionregistrationv1.ParamRef{
						Name:      "allowed-cm",
						Namespace: "default",
					},
				},
			},
		}

		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		baseClient, err := dclient.NewFakeClient(scheme, gvrToListKind, ns, paramCM)
		assert.NilError(t, err)
		baseClient.SetDiscovery(&testNamespacedDiscovery{
			IDiscovery: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
				{Group: "", Version: "v1", Resource: "namespaces"},
				{Group: "", Version: "v1", Resource: "configmaps"},
			}),
			namespaced: true,
		})
		client := &testClientWithKube{
			Interface:  baseClient,
			kubeClient: kubefake.NewSimpleClientset(ns),
		}

		er, err := processVAPWithClient(policy, bindings, *resource, "default", ns, client, a, "default/ConfigMap/target-cm")
		assert.NilError(t, err)
		assert.Equal(t, len(er.PolicyResponse.Rules), 2)
		assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
		assert.Equal(t, er.PolicyResponse.Rules[0].ValidatingAdmissionPolicyBinding().GetName(), "binding-1")
		assert.Equal(t, er.PolicyResponse.Rules[1].Status(), engineapi.RuleStatusPass)
		assert.Equal(t, er.PolicyResponse.Rules[1].ValidatingAdmissionPolicyBinding().GetName(), "binding-2")
		assert.Assert(t, er.IsSuccessful())
	})
}
