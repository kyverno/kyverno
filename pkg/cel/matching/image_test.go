package matching

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Match(t *testing.T) {
	tests := []struct {
		name                 string
		MatchImageReferences []v1alpha1.MatchImageReference
		image                string
		wantResult           bool
		wantErr              bool
	}{{
		name: "standard pass",
		MatchImageReferences: []v1alpha1.MatchImageReference{
			{
				Glob: "ghcr.io/*",
			},
			{
				Expression: "ref == \"ghcr.io/kyverno/kyverno\"",
			},
		},
		image:      "ghcr.io/kyverno/kyverno",
		wantResult: true,
		wantErr:    false,
	}, {
		name: "standard fail",
		MatchImageReferences: []v1alpha1.MatchImageReference{
			{
				Glob: "ghcr.io/*",
			},
			{
				Expression: "ref == \"ghcr.io/kyverno/kyverno\"",
			},
		},
		image:      "kyverno/kyverno",
		wantResult: false,
		wantErr:    false,
	}, {
		name: "second rule matches",
		MatchImageReferences: []v1alpha1.MatchImageReference{
			{
				Glob: "index.docker.io/*",
			},
			{
				Expression: "ref == \"ghcr.io/kyverno/kyverno\"",
			},
		},
		image:      "ghcr.io/kyverno/kyverno",
		wantResult: true,
		wantErr:    false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := compiler.NewMatchImageEnv()
			assert.NoError(t, err)
			c, errList := compiler.CompileMatchImageReferences(field.NewPath("spec", "matchImageReferences"), env, tt.MatchImageReferences...)
			assert.Nil(t, errList)
			matched, err := MatchImage(tt.image, c...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, matched)
			}
		})
	}
}
