package main

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNoLegacyExecutionEscapeHatch proves that this binary's full import graph -- everything
// main.go wires, including every init() along the way -- never registers the legacy execution
// escape hatch or a transitional subresource allowance. It runs as part of package main, so if
// any file in this binary's dependency tree still called deprecations.RegisterLegacyExecutionEscapeHatch
// or deprecations.AllowSubresourceForLegacyStatusWriter, this would observe it. A
// status-subresource update to a legacy kind must be denied, not delegated.
func TestNoLegacyExecutionEscapeHatch(t *testing.T) {
	decision, err := deprecations.DecideLegacyWrite(admissionv1.AdmissionRequest{
		Kind:        metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
		Operation:   admissionv1.Update,
		SubResource: "status",
	})
	assert.Equal(t, deprecations.Deny, decision)
	require.Error(t, err)
}
