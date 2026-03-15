package validation

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestNeedsReports(t *testing.T) {
	dryRunTrue := true
	dryRunFalse := false

	resourceWithUID := unstructured.Unstructured{}
	resourceWithUID.SetUID(types.UID("12345-abcde"))

	resourceWithoutUID := unstructured.Unstructured{}

	tests := []struct {
		name            string
		request         handlers.AdmissionRequest
		resource        unstructured.Unstructured
		admissionReport bool
		setup           func()
		want            bool
	}{
		{
			name: "Happy Path - Reports Needed",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: true,
			setup: func() {
				// Reset and initialize the global config to enable validation reports
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: true,
		},
		{
			name: "Admission Report explicitly false",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: false,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: false,
		},
		{
			name: "Is Dry Run",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunTrue,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: true,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: false,
		},
		{
			name: "Validate Reports Disabled in Global Config",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: true,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil)
			},
			want: false,
		},
		{
			name: "Operation is Delete",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: true,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: false,
		},
		{
			name: "Unsupported GVK - Banned Owner (Event)",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "", // corev1
						Version: "v1",
						Kind:    "Event", // Matches the bannedOwners map
					},
				},
			},
			resource:        resourceWithUID,
			admissionReport: true,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: false,
		},
		{
			name: "Resource has no UID",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					DryRun:    &dryRunFalse,
					Kind: metav1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			resource:        resourceWithoutUID,
			admissionReport: true,
			setup: func() {
				report.ReportingCfg = nil
				report.NewReportingConfig(nil, "validate")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got := NeedsReports(tt.request, tt.resource, tt.admissionReport)
			assert.Equal(t, tt.want, got)
		})
	}
}
