package match

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_Match(t *testing.T) {
	tests := []struct {
		name       string
		imageRules []v1alpha1.ImageRule
		image      string
		wantResult bool
		wantErr    bool
	}{
		{
			name: "standard pass",
			imageRules: []v1alpha1.ImageRule{
				{
					Glob: "ghcr.io/*",
				},
				{
					CELExpression: "ref == \"ghcr.io/kyverno/kyverno\"",
				},
			},
			image:      "ghcr.io/kyverno/kyverno",
			wantResult: true,
			wantErr:    false,
		},
		{
			name: "standard fail",
			imageRules: []v1alpha1.ImageRule{
				{
					Glob: "ghcr.io/*",
				},
				{
					CELExpression: "ref == \"ghcr.io/kyverno/kyverno\"",
				},
			},
			image:      "kyverno/kyverno",
			wantResult: false,
			wantErr:    false,
		},
		{
			name: "second rule matches",
			imageRules: []v1alpha1.ImageRule{
				{
					Glob: "index.docker.io/*",
				},
				{
					CELExpression: "ref == \"ghcr.io/kyverno/kyverno\"",
				},
			},
			image:      "ghcr.io/kyverno/kyverno",
			wantResult: true,
			wantErr:    false,
		},
		{
			name: "invalid cel expression",
			imageRules: []v1alpha1.ImageRule{
				{
					CELExpression: "\"foo\"",
				},
			},
			image:      "ghcr.io/kyverno/kyverno",
			wantResult: false,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := CompiledMatches(tt.imageRules)
			assert.NoError(t, err)
			matched, err := Match(c, tt.image)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, matched)
			}
		})
	}
}
