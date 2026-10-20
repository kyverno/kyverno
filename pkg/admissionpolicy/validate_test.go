package admissionpolicy

import (
	"reflect"
	"testing"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
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
