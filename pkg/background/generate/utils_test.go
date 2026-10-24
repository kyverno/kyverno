package generate

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
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
