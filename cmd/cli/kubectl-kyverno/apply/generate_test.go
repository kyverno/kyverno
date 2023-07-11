package apply

import (
	"reflect"
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_mergeClusterReport(t *testing.T) {
	clustered := []policyreportv1alpha2.ClusterPolicyReport{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterPolicyReport",
				APIVersion: report.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "cpolr-4",
			},
			Results: []policyreportv1alpha2.PolicyReportResult{
				{
					Policy: "cpolr-4",
					Result: report.StatusFail,
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterPolicyReport",
				APIVersion: report.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "cpolr-5",
			},
			Results: []policyreportv1alpha2.PolicyReportResult{
				{
					Policy: "cpolr-5",
					Result: report.StatusFail,
				},
			},
		},
	}

	namespaced := []policyreportv1alpha2.PolicyReport{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyReport",
				APIVersion: report.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ns-polr-1",
				Namespace: "ns-polr",
			},
			Results: []policyreportv1alpha2.PolicyReportResult{
				{
					Policy:    "ns-polr-1",
					Result:    report.StatusPass,
					Resources: make([]corev1.ObjectReference, 10),
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyReport",
				APIVersion: report.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns-polr-2",
			},
			Results: []policyreportv1alpha2.PolicyReportResult{
				{
					Policy:    "ns-polr-2",
					Result:    report.StatusPass,
					Resources: make([]corev1.ObjectReference, 5),
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyReport",
				APIVersion: report.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "polr-3",
			},
			Results: []policyreportv1alpha2.PolicyReportResult{
				{
					Policy:    "polr-3",
					Result:    report.StatusPass,
					Resources: make([]corev1.ObjectReference, 1),
				},
			},
		},
	}

	expectedResults := []policyreportv1alpha2.PolicyReportResult{
		{
			Policy: "cpolr-4",
			Result: report.StatusFail,
		},
		{
			Policy: "cpolr-5",
			Result: report.StatusFail,
		},
		{
			Policy:    "ns-polr-2",
			Result:    report.StatusPass,
			Resources: make([]corev1.ObjectReference, 5),
		},
		{
			Policy:    "polr-3",
			Result:    report.StatusPass,
			Resources: make([]corev1.ObjectReference, 1),
		},
	}

	cpolr := mergeClusterReport(clustered, namespaced)

	assert.Assert(t, cpolr.APIVersion == report.SchemeGroupVersion.String(), cpolr.Kind)
	assert.Assert(t, cpolr.Kind == "ClusterPolicyReport", cpolr.Kind)

	assert.Assert(t, reflect.DeepEqual(cpolr.Results, expectedResults), cpolr.Results)

	assert.Assert(t, cpolr.Summary.Pass == 2, cpolr.Summary.Pass)
	assert.Assert(t, cpolr.Summary.Fail == 2, cpolr.Summary.Fail)
}
