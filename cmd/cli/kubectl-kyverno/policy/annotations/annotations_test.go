package annotations

import (
	"reflect"
	"testing"

	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/openreports"
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
		want        openreportsv1alpha1.ResultSeverity
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
		want: openreports.SeverityCritical,
	}, {
		name: "high",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "high",
		},
		want: openreports.SeverityHigh,
	}, {
		name: "medium",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "medium",
		},
		want: openreports.SeverityMedium,
	}, {
		name: "low",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "low",
		},
		want: openreports.SeverityLow,
	}, {
		name: "info",
		annotations: map[string]string{
			kyverno.AnnotationPolicySeverity: "info",
		},
		want: openreports.SeverityInfo,
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
