package admissionpolicy

import (
	"encoding/json"
	"strings"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// userInfoExpression mirrors GKE's built-in validating-node-p4sa-audience VAP, which references
// request.userInfo.username directly. Background scans have no admission user, so before the fix
// the username key was dropped (omitempty) and evaluation failed with "no such key: username".
const userInfoExpression = `![ "system:addon-manager", "system:serviceaccount:kube-system:cronjob-controller" ].exists(sa, sa == request.userInfo.username)`

// hasUserInfoError reports whether any rule failed with the missing-username evaluation error.
func hasUserInfoError(err error, resp engineapi.EngineResponse) bool {
	if err != nil && strings.Contains(err.Error(), "no such key: username") {
		return true
	}
	for _, r := range resp.PolicyResponse.Rules {
		if strings.Contains(r.Message(), "no such key: username") {
			return true
		}
	}
	return false
}

func TestValidate_BackgroundScan_UserInfoUsernameIsAvailable(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion": "v1", "kind": "Node", "metadata": {"name": "node-1"}}`))
	assert.NilError(t, err)

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "validating-node-p4sa-audience"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: userInfoExpression}},
		},
	}
	policyData := engineapi.NewValidatingAdmissionPolicyData(policy)
	nodeGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}

	// The reports controller passes a non-nil but empty UserInfo for background scans.
	resp, err := Validate(policyData, *resource, resource.GroupVersionKind(), nodeGVR, map[string]map[string]string{}, nil, &authenticationv1.UserInfo{}, true)
	assert.NilError(t, err)
	assert.Assert(t, !hasUserInfoError(err, resp), "request.userInfo.username must be available during background scan")
	assert.Equal(t, 1, len(resp.PolicyResponse.Rules))
	assert.Equal(t, engineapi.RuleStatusPass, resp.PolicyResponse.Rules[0].Status())

	// A nil UserInfo (the other background-scan entry point) must behave the same.
	resp, err = Validate(policyData, *resource, resource.GroupVersionKind(), nodeGVR, map[string]map[string]string{}, nil, nil, true)
	assert.NilError(t, err)
	assert.Assert(t, !hasUserInfoError(err, resp), "request.userInfo.username must be available for nil UserInfo too")
}

func TestMutate_BackgroundScan_UserInfoUsernameIsAvailable(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": {"name": "cm-1", "namespace": "default"}, "data": {"a": "b"}
	}`))
	assert.NilError(t, err)

	rawPolicy := []byte(`{
		"apiVersion": "admissionregistration.k8s.io/v1alpha1",
		"kind": "MutatingAdmissionPolicy",
		"metadata": {"name": "mutate-userinfo"},
		"spec": {
			"matchConstraints": {"resourceRules": [{"apiGroups": [""], "apiVersions": ["v1"], "operations": ["CREATE"], "resources": ["configmaps"]}]},
			"matchConditions": [{"name": "not-system", "expression": "request.userInfo.username != \"system:serviceaccount:kube-system:cronjob-controller\""}],
			"failurePolicy": "Fail",
			"mutations": [{"patchType": "ApplyConfiguration", "applyConfiguration": {"expression": "Object{ metadata: Object.metadata{ labels: {\"scanned\": \"true\"}}}"}}]
		}
	}`)
	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal(rawPolicy, &policy))
	policyData := engineapi.NewMutatingAdmissionPolicyData(&policy)
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	// Empty UserInfo (background scan): the matchCondition references request.userInfo.username,
	// which must be available so evaluation does not error.
	resp, err := Mutate(policyData, *resource, resource.GroupVersionKind(), cmGVR, map[string]map[string]string{}, nil, &authenticationv1.UserInfo{}, true, true)
	assert.NilError(t, err)
	assert.Assert(t, !hasUserInfoError(err, resp), "request.userInfo.username must be available for MAP background scan")
}

// noSuchKey reports whether any rule failed with a "no such key: <field>" evaluation error,
// which is what happens when an omitempty userInfo field is dropped during a background scan.
func noSuchKey(field string, err error, resp engineapi.EngineResponse) bool {
	want := "no such key: " + field
	if err != nil && strings.Contains(err.Error(), want) {
		return true
	}
	for _, r := range resp.PolicyResponse.Rules {
		if strings.Contains(r.Message(), want) {
			return true
		}
	}
	return false
}

// The same omitempty drop that hid request.userInfo.username also hides groups and uid during
// background scans (#14281 residual). A policy referencing either must evaluate without a
// "no such key" error when the scanner passes an empty UserInfo.
func TestValidate_BackgroundScan_GroupsAndUIDAreAvailable(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion": "v1", "kind": "Node", "metadata": {"name": "node-1"}}`))
	assert.NilError(t, err)
	nodeGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}

	cases := []struct {
		field      string
		expression string
	}{
		// the standard "exclude kubelet requests" shape from the Kubernetes VAP docs
		{"groups", `!("system:masters" in request.userInfo.groups)`},
		{"uid", `request.userInfo.uid != "known-uid"`},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "ref-" + tc.field},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					Validations: []admissionregistrationv1.Validation{{Expression: tc.expression}},
				},
			}
			policyData := engineapi.NewValidatingAdmissionPolicyData(policy)

			resp, err := Validate(policyData, *resource, resource.GroupVersionKind(), nodeGVR, map[string]map[string]string{}, nil, &authenticationv1.UserInfo{}, true)
			assert.NilError(t, err)
			assert.Assert(t, !noSuchKey(tc.field, err, resp), "request.userInfo.%s must be available during background scan", tc.field)
			assert.Equal(t, engineapi.RuleStatusPass, resp.PolicyResponse.Rules[0].Status(), "the synthetic scan user must not be a privileged group / known uid")
		})
	}
}

func TestResolveUser_DefaultsGroupsAndUIDWhenEmpty(t *testing.T) {
	u := ResolveUser(&authenticationv1.UserInfo{})
	assert.Assert(t, len(u.GetGroups()) > 0, "groups must be defaulted so request.userInfo.groups is present")
	assert.Assert(t, u.GetUID() != "", "uid must be defaulted so request.userInfo.uid is present")
	// the synthetic group must not be a real privileged group
	for _, g := range u.GetGroups() {
		assert.Assert(t, g != "system:masters" && g != "system:authenticated", "synthetic group must not be privileged: %s", g)
	}
}

func TestResolveUser_PreservesProvidedFieldsWhenUsernameEmpty(t *testing.T) {
	// A caller may supply groups/uid/extra without a username (the CLI accepts a userInfo with
	// only groups). Those fields must be kept; only the username is defaulted.
	provided := &authenticationv1.UserInfo{
		Groups: []string{"system:masters"},
		UID:    "uid-1",
		Extra:  map[string]authenticationv1.ExtraValue{"scopes": {"a"}},
	}
	u := ResolveUser(provided)
	assert.Assert(t, u.GetName() != "", "username must be defaulted")
	assert.DeepEqual(t, []string{"system:masters"}, u.GetGroups())
	assert.Equal(t, "uid-1", u.GetUID())
	assert.DeepEqual(t, []string{"a"}, u.GetExtra()["scopes"])
}

func TestValidate_RealUserInfoIsPreserved(t *testing.T) {
	resource, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion": "v1", "kind": "Node", "metadata": {"name": "node-1"}}`))
	assert.NilError(t, err)

	// Deny when the requester is the specific system account; a real username must be honored,
	// not replaced by the background sentinel.
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-specific-user"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: `request.userInfo.username != "system:serviceaccount:kube-system:cronjob-controller"`}},
		},
	}
	policyData := engineapi.NewValidatingAdmissionPolicyData(policy)
	nodeGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}

	realUser := &authenticationv1.UserInfo{Username: "system:serviceaccount:kube-system:cronjob-controller"}
	resp, err := Validate(policyData, *resource, resource.GroupVersionKind(), nodeGVR, map[string]map[string]string{}, nil, realUser, true)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(resp.PolicyResponse.Rules))
	assert.Equal(t, engineapi.RuleStatusFail, resp.PolicyResponse.Rules[0].Status(), "the real username must be used, so the policy fails")
}
