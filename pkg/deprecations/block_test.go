package deprecations

import (
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func allowHatch() bool          { return true }
func denyNothing(string) bool   { return false }
func allowStatus(s string) bool { return s == "status" }

func terminatingRaw(finalizers string) []byte {
	return []byte(`{"metadata":{"name":"test","deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":` + finalizers + `}}`)
}

func TestDenyLegacyWriteCore(t *testing.T) {
	clusterPolicyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}
	policyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "Policy"}
	legacyExceptionKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "PolicyException"}
	celExceptionKind := metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1", Kind: "PolicyException"}
	cleanupPolicyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "CleanupPolicy"}
	clusterCleanupPolicyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "ClusterCleanupPolicy"}
	validatingPolicyKind := metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1beta1", Kind: "ValidatingPolicy"}
	globalContextKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"}
	updateRequestKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "UpdateRequest"}
	podKind := metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	tests := []struct {
		name               string
		kind               metav1.GroupVersionKind
		operation          admissionv1.Operation
		subResource        string
		oldObject, object  []byte
		escapeHatch        func() bool
		subresourceAllowed func(string) bool
		wantDeny           bool
		wantRecovery       bool
	}{
		{name: "create ClusterPolicy denied", kind: clusterPolicyKind, operation: admissionv1.Create, wantDeny: true},
		{name: "create Policy denied", kind: policyKind, operation: admissionv1.Create, wantDeny: true},
		{name: "create legacy PolicyException denied", kind: legacyExceptionKind, operation: admissionv1.Create, wantDeny: true},
		{name: "create CleanupPolicy denied", kind: cleanupPolicyKind, operation: admissionv1.Create, wantDeny: true},
		{name: "create ClusterCleanupPolicy denied", kind: clusterCleanupPolicyKind, operation: admissionv1.Create, wantDeny: true},
		{name: "spec-changing update denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"spec":{"a":1}}`), object: []byte(`{"spec":{"a":2}}`), wantDeny: true},
		{name: "identical no-op update denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"spec":{"a":1}}`), object: []byte(`{"spec":{"a":1}}`), wantDeny: true},
		{name: "label-only update denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"metadata":{"labels":{"a":"b"}}}`), object: []byte(`{"metadata":{"labels":{"a":"c"}}}`), wantDeny: true},
		{name: "delete allowed", kind: clusterPolicyKind, operation: admissionv1.Delete, wantDeny: false},
		{name: "connect allowed", kind: clusterPolicyKind, operation: admissionv1.Connect, wantDeny: false},
		{name: "status update denied when not registered", kind: clusterPolicyKind, operation: admissionv1.Update,
			subResource: "status", subresourceAllowed: denyNothing, wantDeny: true},
		{name: "status update allowed when registered", kind: clusterPolicyKind, operation: admissionv1.Update,
			subResource: "status", subresourceAllowed: allowStatus, wantDeny: false},
		{name: "scale update denied even when status registered", kind: clusterPolicyKind, operation: admissionv1.Update,
			subResource: "scale", subresourceAllowed: allowStatus, wantDeny: true},
		{name: "finalizer removal on terminating object allowed", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: terminatingRaw(`["a"]`), object: terminatingRaw(`[]`), wantDeny: false, wantRecovery: true},
		{name: "finalizer removal plus label change denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"metadata":{"deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":["a"],"labels":{"a":"b"}}}`),
			object:    []byte(`{"metadata":{"deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":[],"labels":{"a":"c"}}}`),
			wantDeny:  true},
		{name: "finalizer removal plus spec change denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"metadata":{"deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":["a"]},"spec":{"x":1}}`),
			object:    []byte(`{"metadata":{"deletionTimestamp":"2024-01-01T00:00:00Z","finalizers":[]},"spec":{"x":2}}`),
			wantDeny:  true},
		{name: "finalizer add denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: terminatingRaw(`["a"]`), object: terminatingRaw(`["a","b"]`), wantDeny: true},
		{name: "finalizer removal without deletionTimestamp denied", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: []byte(`{"metadata":{"finalizers":["a"]}}`), object: []byte(`{"metadata":{"finalizers":[]}}`), wantDeny: true},
		{name: "finalizer removal to empty allowed", kind: clusterPolicyKind, operation: admissionv1.Update,
			oldObject: terminatingRaw(`["a","b","c"]`), object: terminatingRaw(`[]`), wantDeny: false},
		{name: "escape hatch allows create", kind: clusterPolicyKind, operation: admissionv1.Create, escapeHatch: allowHatch, wantDeny: false},
		{name: "escape hatch allows status update with nothing registered", kind: clusterPolicyKind, operation: admissionv1.Update,
			subResource: "status", escapeHatch: allowHatch, subresourceAllowed: denyNothing, wantDeny: false},
		{name: "CEL PolicyException (policies.kyverno.io) unaffected", kind: celExceptionKind, operation: admissionv1.Create, wantDeny: false},
		{name: "ValidatingPolicy unaffected", kind: validatingPolicyKind, operation: admissionv1.Create, wantDeny: false},
		{name: "GlobalContextEntry unaffected", kind: globalContextKind, operation: admissionv1.Create, wantDeny: false},
		{name: "UpdateRequest unaffected", kind: updateRequestKind, operation: admissionv1.Create, wantDeny: false},
		{name: "Pod unaffected", kind: podKind, operation: admissionv1.Create, wantDeny: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := admissionv1.AdmissionRequest{
				Kind:        tt.kind,
				Operation:   tt.operation,
				SubResource: tt.subResource,
				OldObject:   runtime.RawExtension{Raw: tt.oldObject},
				Object:      runtime.RawExtension{Raw: tt.object},
			}
			subresourceAllowed := tt.subresourceAllowed
			if subresourceAllowed == nil {
				subresourceAllowed = denyNothing
			}
			decision, err := decideLegacyWrite(request, tt.escapeHatch, subresourceAllowed)
			assert.Equal(t, tt.wantDeny, decision == Deny)
			if tt.wantRecovery {
				assert.Equal(t, AllowRecovery, decision, "finalizer recovery must short-circuit, not delegate")
			}
			if tt.wantDeny {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "removed execution")
				assert.Contains(t, err.Error(), "remains available")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDenyLegacyWriteNoRegistrants is the PR 1 proof that, in a package/binary where nothing
// has registered the escape hatch or a subresource allowance, neither a stale toggle value nor
// a stale env var can bypass denial. pkg/deprecations itself registers nothing.
func TestDenyLegacyWriteNoRegistrants(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
	})
	t.Setenv("FLAG_BLOCK_LEGACY_POLICY_APIS", "false")
	require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse("false"))

	clusterPolicyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}

	decision, err := DecideLegacyWrite(admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Create})
	assert.Equal(t, Deny, decision)
	require.Error(t, err)

	decision, err = DecideLegacyWrite(admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Update, SubResource: "status"})
	assert.Equal(t, Deny, decision)
	require.Error(t, err)
}

func TestBuildKindErrorMessage(t *testing.T) {
	err, ok := BuildKindError("kyverno.io", "v1", "ClusterPolicy")
	require.True(t, ok)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "removed execution"))
	assert.True(t, strings.Contains(err.Error(), "remains available"))
	assert.True(t, IsLegacyPolicyBlockError(err))
}
