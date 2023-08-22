package openapi

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_ValidateMutationPolicy(t *testing.T) {

	tcs := []struct {
		description string
		policy      []byte
		mustSucceed bool
	}{
		{
			description: "Policy with mutating imagePullPolicy Overlay",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy-2"},"spec":{"rules":[{"name":"set-image-pull-policy-2","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(name)":"*","imagePullPolicy":"Always"}]}}}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Policy with mutating imagePullPolicy Overlay, type of value is different (does not throw error since all numbers are also strings according to swagger)",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy-2"},"spec":{"rules":[{"name":"set-image-pull-policy-2","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":80}]}}}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Policy with patches",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"policy-endpoints"},"spec":{"rules":[{"name":"pEP","match":{"resources":{"kinds":["Endpoints"],"selector":{"matchLabels":{"label":"test"}}}},"mutate":{"patches": "[{\"path\":\"/subsets/0/ports/0/port\",\"op\":\"replace\",\"value\":9663},{\"path\":\"/metadata/labels/isMutated\",\"op\":\"add\",\"value\":\"true\"}]}}]" }}`),
			mustSucceed: true,
		},
		{
			description: "Dealing with nested variables",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-ns-access-controls","annotations":{"policies.kyverno.io/category":"Workload Isolation","policies.kyverno.io/description":"Create roles and role bindings for a new namespace"}},"spec":{"background":false,"rules":[{"name":"add-sa-annotation","match":{"resources":{"kinds":["Namespace"]}},"mutate":{"overlay":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName-{{something}}}}"}}}}},{"name":"generate-owner-role","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"ClusterRole","name":"ns-owner-{{request.object.metadata.name{{something}}}}-{{request.userInfo.username}}","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"rules":[{"apiGroups":[""],"resources":["namespaces"],"verbs":["delete"],"resourceNames":["{{request.object.metadata.name}}"]}]}}},{"name":"generate-owner-role-binding","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"ClusterRoleBinding","name":"ns-owner-{{request.object.metadata.name}}-{{request.userInfo.username}}-binding","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"ns-owner-{{request.object.metadata.name}}-{{request.userInfo.username}}"},"subjects":[{"kind":"ServiceAccount","name":"{{serviceAccountName}}","namespace":"{{serviceAccountNamespace}}"}]}}},{"name":"generate-admin-role-binding","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"RoleBinding","name":"ns-admin-{{request.object.metadata.name}}-{{request.userInfo.username}}-binding","namespace":"{{request.object.metadata.name}}","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"admin"},"subjects":[{"kind":"ServiceAccount","name":"{{serviceAccountName}}","namespace":"{{serviceAccountNamespace}}"}]}}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Policy with patchesJson6902 and added element at the beginning of a list",
			policy:      []byte(`{"apiVersion": "kyverno.io/v1","kind": "ClusterPolicy","metadata": {"name": "pe"},"spec": {"rules": [{"name": "pe","match": {"resources": {"kinds": ["Endpoints"]}},"mutate": {"patchesJson6902": "- path: \"/subsets/0/addresses/0\"\n  op: add\n  value: {\"ip\":\"123\"}\n- path: \"/subsets/1/addresses/0\"\n  op: add\n  value: {\"ip\":\"123\"}"}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Policy with patchesJson6902 and added element at the end of a list",
			policy:      []byte(`{"apiVersion": "kyverno.io/v1","kind": "ClusterPolicy","metadata": {"name": "pe"},"spec": {"rules": [{"name": "pe","match": {"resources": {"kinds": ["Endpoints"]}},"mutate": {"patchesJson6902": "- path: \"/subsets/0/addresses/-\"\n  op: add\n  value: {\"ip\":\"123\"}\n- path: \"/subsets/1/addresses/-\"\n  op: add\n  value: {\"ip\":\"123\"}"}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Invalid policy with patchStrategicMerge and new match schema(any)",
			policy:      []byte(`{"apiVersion":"kyverno.io\/v1","kind":"ClusterPolicy","metadata":{"name":"mutate-pod"},"spec":{"rules":[{"name":"mutate-pod","match":{"any":[{"resources":{"kinds":["Pod"]}}]},"mutate":{"patchStrategicMerge":{"spec":{"pod":"incorrect"}}}}]}}`),
			mustSucceed: false,
		},
		{
			description: "Invalid policy with patchStrategicMerge and new match schema(all)",
			policy:      []byte(`{"apiVersion":"kyverno.io\/v1","kind":"ClusterPolicy","metadata":{"name":"mutate-pod"},"spec":{"rules":[{"name":"mutate-pod","match":{"all":[{"resources":{"kinds":["Pod"]}}]},"mutate":{"patchStrategicMerge":{"spec":{"pod":"incorrect"}}}}]}}`),
			mustSucceed: false,
		},
		{
			description: "Valid policy with patchStrategicMerge and new match schema(any)",
			policy:      []byte(`{"apiVersion":"kyverno.io\/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy"},"spec":{"rules":[{"name":"set-image-pull-policy","match":{"any":[{"resources":{"kinds":["Pod"]}}]},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(image)":"*:latest","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Valid policy with patchStrategicMerge and new match schema(all)",
			policy:      []byte(`{"apiVersion":"kyverno.io\/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy"},"spec":{"rules":[{"name":"set-image-pull-policy","match":{"all":[{"resources":{"kinds":["Pod"]}}]},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(image)":"*:latest","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			mustSucceed: true,
		},
		{
			description: "Policy with nested foreach and patchesJson6902",
			policy:      []byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"ClusterPolicy","metadata":{"name":"replace-image-registry"},"spec":{"background":false,"validationFailureAction":"Enforce","rules":[{"name":"replace-dns-suffix","match":{"any":[{"resources":{"kinds":["Ingress"]}}]},"mutate":{"foreach":[{"list":"request.object.spec.tls","foreach":[{"list":"element.hosts","patchesJson6902":"- path: \"/spec/tls/{{elementIndex0}}/hosts/{{elementIndex1}}\"\n  op: replace\n  value: \"{{replace_all('{{element}}', '.foo.com', '.newfoo.com')}}\""}]}]}}]}}`),
			mustSucceed: true,
		},
	}

	o, _ := NewManager(logr.Discard())

	for i, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			policy := v1.ClusterPolicy{}
			_ = json.Unmarshal(tc.policy, &policy)
			var errMessage string
			err := o.ValidatePolicyMutation(&policy)
			if err != nil {
				errMessage = err.Error()
			}
			if tc.mustSucceed {
				assert.NilError(t, err, "\nTestcase [%v] failed: Expected no error, Got error:  %v", i+1, errMessage)
			} else {
				assert.Assert(t, err != nil, "\nTestcase [%v] failed: Expected error to have occurred", i+1)
			}
		})
	}

}

func Test_addDefaultFieldsToSchema(t *testing.T) {
	addingDefaultFieldsToSchema("", []byte(`null`))
	addingDefaultFieldsToSchema("", nil)
}

func Test_matchGVK(t *testing.T) {
	testCases := []struct {
		definitionName string
		gvk            string
		match          bool
	}{

		{
			"io.k8s.api.networking.v1.Ingress",
			"networking.k8s.io/v1/Ingress",
			true,
		},
		{
			"io.k8s.api.extensions.v1beta1.Ingress",
			"extensions/v1beta1/Ingress",
			true,
		},
		{
			"io.crossplane.gcp.iam.v1.ServiceAccount",
			"v1/ServiceAccount",
			false,
		},
		{
			"io.k8s.api.core.v1.Secret",
			"v1/Secret",
			true,
		},
		{
			"io.wgpolicyk8s.v1alpha1.PolicyReport",
			"wgpolicyk8s.io/v1alpha1/PolicyReport",
			true,
		},
		{
			"io.k8s.api.rbac.v1.RoleBinding",
			"rbac.authorization.k8s.io/v1/RoleBinding",
			true,
		},
		{
			"io.k8s.api.rbac.v1beta1.ClusterRoleBinding",
			"rbac.authorization.k8s.io/v1beta1/ClusterRoleBinding",
			true,
		},
		{
			"io.crossplane.gcp.iam.v1alpha1.ServiceAccount",
			"iam.gcp.crossplane.io/v1alpha1/ServiceAccount",
			true,
		},
		{
			"io.crossplane.gcp.iam.v1alpha1.ServiceAccount",
			"v1/ServiceAccount",
			false,
		},
		{
			"v1.ServiceAccount",
			"iam.gcp.crossplane.io/v1alpha1/ServiceAccount",
			false,
		},
		{
			"io.k8s.api.rbac.v1.Role",
			"rbac.authorization.k8s.io/v1/Role",
			true,
		},
		{
			"io.k8s.api.rbac.v1.ClusterRole",
			"rbac.authorization.k8s.io/v1/ClusterRole",
			true,
		},
		{
			"io.k8s.api.flowcontrol.v1beta1.FlowSchema",
			"flowcontrol.apiserver.k8s.io/v1beta1/FlowSchema",
			true,
		},
		{
			"io.k8s.api.policy.v1beta1.Eviction",
			"v1/Eviction",
			false,
		},
		{
			"io.k8s.api.rbac.v1beta1.ClusterRole",
			"rbac.authorization.k8s.io/v1beta1/ClusterRole",
			true,
		},
		{
			"io.k8s.api.policy.v1.Eviction",
			"v1/Eviction",
			false,
		},
	}

	for i, test := range testCases {
		t.Run(test.definitionName, func(t *testing.T) {
			res := matchGVK(test.definitionName, test.gvk)
			assert.Equal(t, res, test.match, "test #%d failed", i)
		})
	}
}

// this test covers all supported Ingress
func Test_Ingress(t *testing.T) {
	o, err := NewManager(logr.Discard())
	assert.NilError(t, err)

	versions, ok := o.kindToAPIVersions.Get("Ingress")
	assert.Equal(t, true, ok)

	assert.Equal(t, versions.serverPreferredGVK, "networking.k8s.io/v1/Ingress")
	assert.Equal(t, len(versions.gvks), 1)

	definitionName, _ := o.gvkToDefinitionName.Get("Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.networking.v1.Ingress")

	definitionName, _ = o.gvkToDefinitionName.Get("networking.k8s.io/v1/Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.networking.v1.Ingress")
}
