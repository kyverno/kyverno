package admissionpolicy

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

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
			_, policy, _, _, _, _, _, err := yamlutils.GetPolicy(tt.policy)
			assert.NilError(t, err)
			restMapper, err := utils.GetRESTMapper(nil)
			assert.NilError(t, err)
			kinds := GetKinds(policy[0].Spec.MatchConstraints, restMapper)
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

func Test_ValidateMultipleBindingsDenyIsNotOverwritten(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {"name": "payments", "namespace": "default", "labels": {"tier": "critical"}},
		"spec": {"replicas": 5}
	}`))
	assert.NilError(t, err)

	policy := admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "max-replicas"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			ParamKind: &admissionregistrationv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.replicas <= int(params.data.maxReplicas)",
			}},
		},
	}

	binding := func(name, param string, selector *metav1.LabelSelector) admissionregistrationv1.ValidatingAdmissionPolicyBinding {
		b := admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName:        policy.Name,
				ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
				ParamRef:          &admissionregistrationv1.ParamRef{Name: param},
			},
		}
		if selector != nil {
			b.Spec.MatchResources = &admissionregistrationv1.MatchResources{ObjectSelector: selector}
		}
		return b
	}
	critical := binding("max-replicas-critical", "critical-limits", &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "critical"}})
	baseline := binding("max-replicas-baseline", "baseline-limits", nil)

	param := func(name, max string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": name},
			"data":       map[string]any{"maxReplicas": max},
		}}
	}

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	client := dclient.NewFakeClientWithDisco(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), param("critical-limits", "3"), param("baseline-limits", "10")),
		kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
		dclient.NewFakeDiscoveryClient(nil),
	)

	for _, tc := range []struct {
		name     string
		bindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
		isFake   bool
	}{
		{name: "denying binding first", bindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{critical, baseline}, isFake: true},
		{name: "denying binding last", bindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{baseline, critical}, isFake: true},
		{name: "denying binding first with client", bindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{critical, baseline}, isFake: false},
		{name: "denying binding last with client", bindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{baseline, critical}, isFake: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := engineapi.NewValidatingAdmissionPolicyData(policy.DeepCopy())
			for _, b := range tc.bindings {
				data.AddBinding(b)
			}
			data.AddParam(param("critical-limits", "3"))
			data.AddParam(param("baseline-limits", "10"))

			response, err := Validate(data, *resource, gvk, gvr, nil, client, nil, tc.isFake)
			assert.NilError(t, err)

			var statuses []engineapi.RuleStatus
			denied := false
			for _, rule := range response.PolicyResponse.Rules {
				statuses = append(statuses, rule.Status())
				if rule.Status() == engineapi.RuleStatusFail {
					denied = true
				}
			}
			assert.Assert(t, denied, "expected a Fail from binding max-replicas-critical, got rule statuses %v", statuses)
		})
	}
}
