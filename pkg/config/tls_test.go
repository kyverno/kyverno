package config

import (
	"testing"
)

func TestInClusterServiceName(t *testing.T) {
	tests := []struct {
		name       string
		commonName string
		namespace  string
		expected   string
	}{
		{
			name:       "standard service name",
			commonName: "kyverno",
			namespace:  "kyverno",
			expected:   "kyverno.kyverno.svc",
		},
		{
			name:       "different namespace",
			commonName: "kyverno-svc",
			namespace:  "default",
			expected:   "kyverno-svc.default.svc",
		},
		{
			name:       "cleanup controller service",
			commonName: "kyverno-cleanup-controller",
			namespace:  "kyverno",
			expected:   "kyverno-cleanup-controller.kyverno.svc",
		},
		{
			name:       "empty common name",
			commonName: "",
			namespace:  "kyverno",
			expected:   ".kyverno.svc",
		},
		{
			name:       "empty namespace",
			commonName: "kyverno",
			namespace:  "",
			expected:   "kyverno..svc",
		},
		{
			name:       "hyphenated names",
			commonName: "kyverno-admission-controller",
			namespace:  "kyverno-system",
			expected:   "kyverno-admission-controller.kyverno-system.svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InClusterServiceName(tt.commonName, tt.namespace)
			if result != tt.expected {
				t.Errorf("InClusterServiceName(%q, %q) = %q, want %q",
					tt.commonName, tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestDnsNames(t *testing.T) {
	tests := []struct {
		name       string
		commonName string
		namespace  string
		expected   []string
	}{
		{
			name:       "standard DNS names",
			commonName: "kyverno",
			namespace:  "kyverno",
			expected: []string{
				"kyverno",
				"kyverno.kyverno",
				"kyverno.kyverno.svc",
			},
		},
		{
			name:       "different namespace",
			commonName: "kyverno-svc",
			namespace:  "default",
			expected: []string{
				"kyverno-svc",
				"kyverno-svc.default",
				"kyverno-svc.default.svc",
			},
		},
		{
			name:       "cleanup controller",
			commonName: "kyverno-cleanup-controller",
			namespace:  "kyverno",
			expected: []string{
				"kyverno-cleanup-controller",
				"kyverno-cleanup-controller.kyverno",
				"kyverno-cleanup-controller.kyverno.svc",
			},
		},
		{
			name:       "reports controller",
			commonName: "kyverno-reports-controller",
			namespace:  "kyverno-system",
			expected: []string{
				"kyverno-reports-controller",
				"kyverno-reports-controller.kyverno-system",
				"kyverno-reports-controller.kyverno-system.svc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DnsNames(tt.commonName, tt.namespace)

			if len(result) != len(tt.expected) {
				t.Fatalf("DnsNames(%q, %q) returned %d names, want %d",
					tt.commonName, tt.namespace, len(result), len(tt.expected))
			}

			for i, name := range result {
				if name != tt.expected[i] {
					t.Errorf("DnsNames(%q, %q)[%d] = %q, want %q",
						tt.commonName, tt.namespace, i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestDnsNamesContainsInClusterServiceName(t *testing.T) {
	commonName := "kyverno"
	namespace := "kyverno"

	dnsNames := DnsNames(commonName, namespace)
	inClusterName := InClusterServiceName(commonName, namespace)

	found := false
	for _, name := range dnsNames {
		if name == inClusterName {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("DnsNames should contain InClusterServiceName %q, but it doesn't. DNS names: %v",
			inClusterName, dnsNames)
	}
}
