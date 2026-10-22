package common

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestTriggerFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   kyvernov1.ResourceSpec
	}{
		{
			name: "core v1 resource",
			labels: map[string]string{
				GenerateTriggerGroupLabel:   "",
				GenerateTriggerVersionLabel: "v1",
				GenerateTriggerKindLabel:    "Pod",
				GenerateTriggerNSLabel:      "default",
				GenerateTriggerNameLabel:    "my-pod",
				GenerateTriggerUIDLabel:     "abc-123",
			},
			want: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "my-pod",
				UID:        types.UID("abc-123"),
			},
		},
		{
			name: "apps group resource",
			labels: map[string]string{
				GenerateTriggerGroupLabel:   "apps",
				GenerateTriggerVersionLabel: "v1",
				GenerateTriggerKindLabel:    "Deployment",
				GenerateTriggerNSLabel:      "kube-system",
				GenerateTriggerNameLabel:    "nginx",
				GenerateTriggerUIDLabel:     "def-456",
			},
			want: kyvernov1.ResourceSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "kube-system",
				Name:       "nginx",
				UID:        types.UID("def-456"),
			},
		},
		{
			name: "cluster-scoped resource",
			labels: map[string]string{
				GenerateTriggerGroupLabel:   "",
				GenerateTriggerVersionLabel: "v1",
				GenerateTriggerKindLabel:    "Namespace",
				GenerateTriggerNSLabel:      "",
				GenerateTriggerNameLabel:    "test-ns",
				GenerateTriggerUIDLabel:     "ghi-789",
			},
			want: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Namespace",
				Namespace:  "",
				Name:       "test-ns",
				UID:        types.UID("ghi-789"),
			},
		},
		{
			name: "custom resource with group",
			labels: map[string]string{
				GenerateTriggerGroupLabel:   "networking.k8s.io",
				GenerateTriggerVersionLabel: "v1",
				GenerateTriggerKindLabel:    "NetworkPolicy",
				GenerateTriggerNSLabel:      "production",
				GenerateTriggerNameLabel:    "deny-all",
				GenerateTriggerUIDLabel:     "jkl-012",
			},
			want: kyvernov1.ResourceSpec{
				APIVersion: "networking.k8s.io/v1",
				Kind:       "NetworkPolicy",
				Namespace:  "production",
				Name:       "deny-all",
				UID:        types.UID("jkl-012"),
			},
		},
		{
			name:   "empty labels",
			labels: map[string]string{},
			want: kyvernov1.ResourceSpec{
				APIVersion: "",
				Kind:       "",
				Namespace:  "",
				Name:       "",
				UID:        types.UID(""),
			},
		},
		{
			name:   "nil labels",
			labels: nil,
			want: kyvernov1.ResourceSpec{
				APIVersion: "",
				Kind:       "",
				Namespace:  "",
				Name:       "",
				UID:        types.UID(""),
			},
		},
		{
			name: "beta version resource",
			labels: map[string]string{
				GenerateTriggerGroupLabel:   "policy",
				GenerateTriggerVersionLabel: "v1beta1",
				GenerateTriggerKindLabel:    "PodDisruptionBudget",
				GenerateTriggerNSLabel:      "default",
				GenerateTriggerNameLabel:    "pdb-test",
				GenerateTriggerUIDLabel:     "mno-345",
			},
			want: kyvernov1.ResourceSpec{
				APIVersion: "policy/v1beta1",
				Kind:       "PodDisruptionBudget",
				Namespace:  "default",
				Name:       "pdb-test",
				UID:        types.UID("mno-345"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TriggerFromLabels(tt.labels)
			assert.Equal(t, tt.want, got)
		})
	}
}
