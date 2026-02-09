package generate

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func Test_newResourceSpec(t *testing.T) {
	tests := []struct {
		name          string
		genAPIVersion string
		genKind       string
		genNamespace  string
		genName       string
		want          kyvernov1.ResourceSpec
	}{
		{
			name:          "core resource",
			genAPIVersion: "v1",
			genKind:       "ConfigMap",
			genNamespace:  "default",
			genName:       "my-config",
			want: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  "default",
				Name:       "my-config",
			},
		},
		{
			name:          "apps group resource",
			genAPIVersion: "apps/v1",
			genKind:       "Deployment",
			genNamespace:  "kube-system",
			genName:       "nginx-deployment",
			want: kyvernov1.ResourceSpec{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "kube-system",
				Name:       "nginx-deployment",
			},
		},
		{
			name:          "cluster-scoped resource",
			genAPIVersion: "v1",
			genKind:       "Namespace",
			genNamespace:  "",
			genName:       "test-namespace",
			want: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Namespace",
				Namespace:  "",
				Name:       "test-namespace",
			},
		},
		{
			name:          "empty values",
			genAPIVersion: "",
			genKind:       "",
			genNamespace:  "",
			genName:       "",
			want: kyvernov1.ResourceSpec{
				APIVersion: "",
				Kind:       "",
				Namespace:  "",
				Name:       "",
			},
		},
		{
			name:          "custom resource",
			genAPIVersion: "kyverno.io/v1",
			genKind:       "ClusterPolicy",
			genNamespace:  "",
			genName:       "require-labels",
			want: kyvernov1.ResourceSpec{
				APIVersion: "kyverno.io/v1",
				Kind:       "ClusterPolicy",
				Namespace:  "",
				Name:       "require-labels",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newResourceSpec(tt.genAPIVersion, tt.genKind, tt.genNamespace, tt.genName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTriggerFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   kyvernov1.ResourceSpec
	}{
		{
			name: "core v1 resource",
			labels: map[string]string{
				common.GenerateTriggerGroupLabel:   "",
				common.GenerateTriggerVersionLabel: "v1",
				common.GenerateTriggerKindLabel:    "Pod",
				common.GenerateTriggerNSLabel:      "default",
				common.GenerateTriggerNameLabel:    "my-pod",
				common.GenerateTriggerUIDLabel:     "abc-123",
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
				common.GenerateTriggerGroupLabel:   "apps",
				common.GenerateTriggerVersionLabel: "v1",
				common.GenerateTriggerKindLabel:    "Deployment",
				common.GenerateTriggerNSLabel:      "kube-system",
				common.GenerateTriggerNameLabel:    "nginx",
				common.GenerateTriggerUIDLabel:     "def-456",
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
				common.GenerateTriggerGroupLabel:   "",
				common.GenerateTriggerVersionLabel: "v1",
				common.GenerateTriggerKindLabel:    "Namespace",
				common.GenerateTriggerNSLabel:      "",
				common.GenerateTriggerNameLabel:    "test-ns",
				common.GenerateTriggerUIDLabel:     "ghi-789",
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
				common.GenerateTriggerGroupLabel:   "networking.k8s.io",
				common.GenerateTriggerVersionLabel: "v1",
				common.GenerateTriggerKindLabel:    "NetworkPolicy",
				common.GenerateTriggerNSLabel:      "production",
				common.GenerateTriggerNameLabel:    "deny-all",
				common.GenerateTriggerUIDLabel:     "jkl-012",
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
				common.GenerateTriggerGroupLabel:   "policy",
				common.GenerateTriggerVersionLabel: "v1beta1",
				common.GenerateTriggerKindLabel:    "PodDisruptionBudget",
				common.GenerateTriggerNSLabel:      "default",
				common.GenerateTriggerNameLabel:    "pdb-test",
				common.GenerateTriggerUIDLabel:     "mno-345",
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
