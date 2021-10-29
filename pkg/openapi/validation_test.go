package openapi

import (
	"encoding/json"
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_ValidateMutationPolicy(t *testing.T) {

	tcs := []struct {
		description string
		policy      []byte
		errMessage  string
	}{
		{
			description: "Policy with mutating imagePullPolicy Overlay",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy-2"},"spec":{"rules":[{"name":"set-image-pull-policy-2","match":{"resources":{"kinds":["Pod"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"Always"}]}}}}]}}`),
		},
		{
			description: "Policy with mutating imagePullPolicy Overlay, field does not exist",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy-2"},"spec":{"rules":[{"name":"set-image-pull-policy-2","match":{"resources":{"kinds":["Pod"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","nonExistantField":"Always"}]}}}}]}}`),
			errMessage:  `ValidationError(io.k8s.api.core.v1.Pod.spec.containers[0]): unknown field "nonExistantField" in io.k8s.api.core.v1.Container`,
		},
		{
			description: "Policy with mutating imagePullPolicy Overlay, type of value is different (does not throw error since all numbers are also strings according to swagger)",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-image-pull-policy-2"},"spec":{"rules":[{"name":"set-image-pull-policy-2","match":{"resources":{"kinds":["Pod"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":80}]}}}}]}}`),
		},
		{
			description: "Policy with patches",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"policy-endpoints"},"spec":{"rules":[{"name":"pEP","match":{"resources":{"kinds":["Endpoints"],"selector":{"matchLabels":{"label":"test"}}}},"mutate":{"patches":[{"path":"/subsets/0/ports/0/port","op":"replace","value":9663},{"path":"/metadata/labels/isMutated","op":"add","value":"true"}]}}]}}`),
		},
		{
			description: "Policy with patches, value converted from number to string",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"policy-endpoints"},"spec":{"rules":[{"name":"pEP","match":{"resources":{"kinds":["Endpoints"],"selector":{"matchLabels":{"label":"test"}}}},"mutate":{"patches":[{"path":"/subsets/0/ports/0/port","op":"replace","value":"9663"},{"path":"/metadata/labels/isMutated","op":"add","value":"true"}]}}]}}`),
			errMessage:  `ValidationError(io.k8s.api.core.v1.Endpoints.subsets[0].ports[0].port): invalid type for io.k8s.api.core.v1.EndpointPort.port: got "string", expected "integer"`,
		},
		{
			description: "Policy where boolean is been converted to number",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"mutate-pod-disable-automoutingapicred"},"spec":{"rules":[{"name":"pod-disable-automoutingapicred","match":{"resources":{"kinds":["Pod"]}},"mutate":{"overlay":{"spec":{"(serviceAccountName)":"*","automountServiceAccountToken":80}}}}]}}`),
			errMessage:  `ValidationError(io.k8s.api.core.v1.Pod.spec.automountServiceAccountToken): invalid type for io.k8s.api.core.v1.PodSpec.automountServiceAccountToken: got "integer", expected "boolean"`,
		},
		{
			description: "Dealing with nested variables",
			policy:      []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-ns-access-controls","annotations":{"policies.kyverno.io/category":"Workload Isolation","policies.kyverno.io/description":"Create roles and role bindings for a new namespace"}},"spec":{"background":false,"rules":[{"name":"add-sa-annotation","match":{"resources":{"kinds":["Namespace"]}},"mutate":{"overlay":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName-{{something}}}}"}}}}},{"name":"generate-owner-role","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"ClusterRole","name":"ns-owner-{{request.object.metadata.name{{something}}}}-{{request.userInfo.username}}","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"rules":[{"apiGroups":[""],"resources":["namespaces"],"verbs":["delete"],"resourceNames":["{{request.object.metadata.name}}"]}]}}},{"name":"generate-owner-role-binding","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"ClusterRoleBinding","name":"ns-owner-{{request.object.metadata.name}}-{{request.userInfo.username}}-binding","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"ns-owner-{{request.object.metadata.name}}-{{request.userInfo.username}}"},"subjects":[{"kind":"ServiceAccount","name":"{{serviceAccountName}}","namespace":"{{serviceAccountNamespace}}"}]}}},{"name":"generate-admin-role-binding","match":{"resources":{"kinds":["Namespace"]}},"preconditions":[{"key":"{{request.userInfo.username}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountName}}","operator":"NotEqual","value":""},{"key":"{{serviceAccountNamespace}}","operator":"NotEqual","value":""}],"generate":{"kind":"RoleBinding","name":"ns-admin-{{request.object.metadata.name}}-{{request.userInfo.username}}-binding","namespace":"{{request.object.metadata.name}}","data":{"metadata":{"annotations":{"nirmata.io/ns-creator":"{{serviceAccountName}}"}},"roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"admin"},"subjects":[{"kind":"ServiceAccount","name":"{{serviceAccountName}}","namespace":"{{serviceAccountNamespace}}"}]}}}]}}`),
		},
		{
			description: "Policy with patchesJson6902 and added element at the beginning of a list",
			policy:      []byte(`{"apiVersion": "kyverno.io/v1","kind": "ClusterPolicy","metadata": {"name": "pe"},"spec": {"rules": [{"name": "pe","match": {"resources": {"kinds": ["Endpoints"]}},"mutate": {"patchesJson6902": "- path: \"/subsets/0/addresses/0\"\n  op: add\n  value: {\"ip\":\"123\"}\n- path: \"/subsets/1/addresses/0\"\n  op: add\n  value: {\"ip\":\"123\"}"}}]}}`),
		},
		{
			description: "Policy with patchesJson6902 and added element at the end of a list",
			policy:      []byte(`{"apiVersion": "kyverno.io/v1","kind": "ClusterPolicy","metadata": {"name": "pe"},"spec": {"rules": [{"name": "pe","match": {"resources": {"kinds": ["Endpoints"]}},"mutate": {"patchesJson6902": "- path: \"/subsets/0/addresses/-\"\n  op: add\n  value: {\"ip\":\"123\"}\n- path: \"/subsets/1/addresses/-\"\n  op: add\n  value: {\"ip\":\"123\"}"}}]}}`),
		},
	}

	o, _ := NewOpenAPIController()

	for i, tc := range tcs {
		policy := v1.ClusterPolicy{}
		_ = json.Unmarshal(tc.policy, &policy)

		var errMessage string
		err := o.ValidatePolicyMutation(policy)
		if err != nil {
			errMessage = err.Error()
		}

		if errMessage != tc.errMessage {
			t.Errorf("\nTestcase [%v] failed:\nExpected Error:  %v\nGot Error:  %v", i+1, tc.errMessage, errMessage)
		}
	}

}

func Test_addDefaultFieldsToSchema(t *testing.T) {
	addingDefaultFieldsToSchema([]byte(`null`))
	addingDefaultFieldsToSchema(nil)
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
			true,
		},
		{
			"io.k8s.api.rbac.v1beta1.ClusterRole",
			"rbac.authorization.k8s.io/v1beta1/ClusterRole",
			true,
		},
	}

	for i, test := range testCases {
		res := matchGVK(test.definitionName, test.gvk)
		assert.Equal(t, res, test.match, "test #%d failed", i)
	}
}

// this test covers all supported Ingress in 1.20 cluster
// networking.k8s.io/v1/Ingress
// networking.k8s.io/v1beta1/Ingress
// extensions/v1beta1/Ingress
func Test_Ingress(t *testing.T) {
	o, err := NewOpenAPIController()
	assert.NilError(t, err)

	versions, ok := o.kindToAPIVersions.Get("Ingress")
	assert.Equal(t, true, ok)
	versionsTyped := versions.(apiVersions)
	assert.Equal(t, versionsTyped.serverPreferredGVK, "networking.k8s.io/v1/Ingress")
	assert.Equal(t, len(versionsTyped.gvks), 3)

	definitionName, _ := o.gvkToDefinitionName.Get("Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.networking.v1.Ingress")

	definitionName, _ = o.gvkToDefinitionName.Get("networking.k8s.io/v1/Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.networking.v1.Ingress")

	definitionName, _ = o.gvkToDefinitionName.Get("networking.k8s.io/v1beta1/Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.networking.v1beta1.Ingress")

	definitionName, _ = o.gvkToDefinitionName.Get("extensions/v1beta1/Ingress")
	assert.Equal(t, definitionName, "io.k8s.api.extensions.v1beta1.Ingress")
}
