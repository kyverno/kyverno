package annotations

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

func TestScored(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{{
		name:        "nil",
		annotations: nil,
		want:        true,
	}, {
		name:        "empty",
		annotations: map[string]string{},
		want:        true,
	}, {
		name: "not present",
		annotations: map[string]string{
			"foo": "bar",
		},
		want: true,
	}, {
		name: "false",
		annotations: map[string]string{
			kyverno.AnnotationPolicyScored: "false",
		},
		want: false,
	}, {
		name: "true",
		annotations: map[string]string{
			kyverno.AnnotationPolicyScored: "true",
		},
		want: true,
	}, {
		name: "bar",
		annotations: map[string]string{
			kyverno.AnnotationPolicyScored: "bar",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Scored(tt.annotations); got != tt.want {
				t.Errorf("Scored() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverity(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        policyreportv1alpha2.PolicySeverity
	}{{
		name:        "nil",
		annotations: nil,
		want:        "",
	}, {
		name:        "empty",
		annotations: map[string]string{},
		want:        "",
	}, {
		name: "not present",
		annotations: map[string]string{
			"foo": "bar",
		},
		want: "",
	}, {
		name: "critical",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "critical",
		},
		want: policyreportv1alpha2.SeverityCritical,
	}, {
		name: "high",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "high",
		},
		want: policyreportv1alpha2.SeverityHigh,
	}, {
		name: "medium",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "medium",
		},
		want: policyreportv1alpha2.SeverityMedium,
	}, {
		name: "low",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "low",
		},
		want: policyreportv1alpha2.SeverityLow,
	}, {
		name: "info",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "info",
		},
		want: policyreportv1alpha2.SeverityInfo,
	}, {
		name: "bar",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "bar",
		},
		want: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Severity(tt.annotations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Severity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategory(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
	}{{
		name:        "nil",
		annotations: nil,
		want:        "",
	}, {
		name:        "empty",
		annotations: map[string]string{},
		want:        "",
	}, {
		name: "not present",
		annotations: map[string]string{
			"foo": "bar",
		},
		want: "",
	}, {
		name: "category",
		annotations: map[string]string{
			kyverno.AnnotationPolicyCategory: "category",
		},
		want: "category",
	}, {
		name: "set to empty",
		annotations: map[string]string{
			kyverno.AnnotationPolicyCategory: "",
		},
		want: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Category(tt.annotations); got != tt.want {
				t.Errorf("Category() = %v, want %v", got, tt.want)
			}
		})
	}
}
