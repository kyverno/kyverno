package report

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPolicyReportable_NilInterface(t *testing.T) {
	// A bare nil interface must return false.
	assert.False(t, IsPolicyReportable(nil))
}

func TestIsPolicyReportable_TypedNilPointer(t *testing.T) {
	// A typed nil (*kyvernov1.ClusterPolicy)(nil) stored in a metav1.Object
	// interface is NOT equal to nil — the old guard missed this case and
	// would panic on pol.GetLabels(). The fix must return false instead.
	var cp *kyvernov1.ClusterPolicy
	assert.False(t, IsPolicyReportable(cp), "typed nil pointer must return false without panicking")
}

func TestIsPolicyReportable_TypedNilPolicy(t *testing.T) {
	// Same scenario via the namespaced Policy type.
	var p *kyvernov1.Policy
	assert.False(t, IsPolicyReportable(p), "typed nil *Policy must return false without panicking")
}

func TestIsPolicyReportable_ValidPolicyNoLabel(t *testing.T) {
	// A fully initialised policy with no exclude-reporting label must return true.
	cp := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}
	assert.True(t, IsPolicyReportable(cp))
}

func TestIsPolicyReportable_ValidPolicyWithExcludeLabel(t *testing.T) {
	// A policy carrying the LabelExcludeReporting label must return false.
	cp := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
			Labels: map[string]string{
				"reports.kyverno.io/disabled": "true",
			},
		},
	}
	assert.False(t, IsPolicyReportable(cp))
}

func TestIsPolicyReportable_ValidPolicyEmptyLabels(t *testing.T) {
	// Initialised policy with an explicit but empty label map must return true.
	cp := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-policy",
			Labels: map[string]string{},
		},
	}
	assert.True(t, IsPolicyReportable(cp))
}
