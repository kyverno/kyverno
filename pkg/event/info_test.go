package event

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestInfoResource(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name: "namespaced resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-pod",
				},
			},
			expected: "Pod/default/my-pod",
		},
		{
			name: "cluster-scoped resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "ClusterPolicy",
					Namespace: "",
					Name:      "require-labels",
				},
			},
			expected: "ClusterPolicy/require-labels",
		},
		{
			name: "deployment resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Deployment",
					Namespace: "kyverno",
					Name:      "kyverno-admission-controller",
				},
			},
			expected: "Deployment/kyverno/kyverno-admission-controller",
		},
		{
			name: "namespace resource (cluster-scoped)",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Namespace",
					Namespace: "",
					Name:      "production",
				},
			},
			expected: "Namespace/production",
		},
		{
			name: "configmap resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "ConfigMap",
					Namespace: "kube-system",
					Name:      "coredns",
				},
			},
			expected: "ConfigMap/kube-system/coredns",
		},
		{
			name: "secret resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Secret",
					Namespace: "default",
					Name:      "my-secret",
				},
			},
			expected: "Secret/default/my-secret",
		},
		{
			name: "service resource",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Service",
					Namespace: "monitoring",
					Name:      "prometheus",
				},
			},
			expected: "Service/monitoring/prometheus",
		},
		{
			name: "cluster role (cluster-scoped)",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "ClusterRole",
					Namespace: "",
					Name:      "admin",
				},
			},
			expected: "ClusterRole/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.Resource()
			if result != tt.expected {
				t.Errorf("Info.Resource() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInfoResourceWithEmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name: "empty kind and name",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "",
					Namespace: "",
					Name:      "",
				},
			},
			expected: "/",
		},
		{
			name: "empty name only",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "",
				},
			},
			expected: "Pod/default/",
		},
		{
			name: "empty kind only",
			info: Info{
				Regarding: corev1.ObjectReference{
					Kind:      "",
					Namespace: "",
					Name:      "my-resource",
				},
			},
			expected: "/my-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.Resource()
			if result != tt.expected {
				t.Errorf("Info.Resource() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInfoStructFields(t *testing.T) {
	related := &corev1.ObjectReference{
		Kind:      "Policy",
		Namespace: "kyverno",
		Name:      "require-labels",
	}

	info := Info{
		Regarding: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
		},
		Related: related,
		Reason:  PolicyViolation,
		Message: "Pod does not have required labels",
		Action:  None,
		Source:  AdmissionController,
		Type:    "Warning",
	}

	// Verify struct fields are correctly set
	if info.Reason != PolicyViolation {
		t.Errorf("Expected Reason = PolicyViolation, got %v", info.Reason)
	}

	if info.Message != "Pod does not have required labels" {
		t.Errorf("Unexpected Message: %s", info.Message)
	}

	if info.Type != "Warning" {
		t.Errorf("Expected Type = Warning, got %s", info.Type)
	}

	if info.Related == nil {
		t.Error("Related should not be nil")
	}

	if info.Related.Kind != "Policy" {
		t.Errorf("Expected Related.Kind = Policy, got %s", info.Related.Kind)
	}
}

func TestReasonConstants(t *testing.T) {
	// Verify reason constants are defined correctly
	if PolicyViolation != "PolicyViolation" {
		t.Errorf("PolicyViolation = %q, want %q", PolicyViolation, "PolicyViolation")
	}
	if PolicyApplied != "PolicyApplied" {
		t.Errorf("PolicyApplied = %q, want %q", PolicyApplied, "PolicyApplied")
	}
	if PolicyError != "PolicyError" {
		t.Errorf("PolicyError = %q, want %q", PolicyError, "PolicyError")
	}
	if PolicySkipped != "PolicySkipped" {
		t.Errorf("PolicySkipped = %q, want %q", PolicySkipped, "PolicySkipped")
	}
}
