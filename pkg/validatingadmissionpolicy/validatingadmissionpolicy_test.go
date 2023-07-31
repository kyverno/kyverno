package validatingadmissionpolicy

import (
	"reflect"
	"testing"

	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
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
apiVersion: admissionregistration.k8s.io/v1alpha1
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
			name: "Matching deployments, replicasets, daemonsets and statefulsets",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
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
			wantKinds: []string{"apps/v1/Deployment", "apps/v1/Replicaset", "apps/v1/Daemonset", "apps/v1/Statefulset"},
		},
		{
			name: "Matching deployments/scale",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
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
apiVersion: admissionregistration.k8s.io/v1alpha1
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
			wantKinds: []string{"batch/v1/Job", "batch/v1/Cronjob"},
		},
		{
			name: "Multiple resource rules",
			policy: []byte(`
apiVersion: admissionregistration.k8s.io/v1alpha1
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
			wantKinds: []string{"v1/Pod", "apps/v1/Deployment", "apps/v1/Replicaset", "apps/v1/Daemonset", "apps/v1/Statefulset", "batch/v1/Job", "batch/v1/Cronjob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, policy, _ := yamlutils.GetPolicy(tt.policy)
			kinds := GetKinds(policy[0])
			if !reflect.DeepEqual(kinds, tt.wantKinds) {
				t.Errorf("Expected %v, got %v", tt.wantKinds, kinds)
			}
		})
	}
}
