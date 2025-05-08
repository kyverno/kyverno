package compiler

import (
	"testing"

	"github.com/gobwas/glob"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_matchGlob_Match(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		image   string
		want    bool
		wantErr bool
	}{{
		name:  "match",
		glob:  "ghcr.io/*",
		image: "ghcr.io/kyverno/kyverno",
		want:  true,
	}, {
		name:  "not match",
		glob:  "ghcr.io/*/foo",
		image: "ghcr.io/kyverno/kyverno",
		want:  false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &matchGlob{
				Glob: glob.MustCompile(tt.glob),
			}
			got, err := m.Match(tt.image)
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_matchCel_Match(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		image      string
		want       bool
		wantErr    bool
	}{{
		name:       "match",
		expression: `ref == "ghcr.io/kyverno/kyverno"`,
		image:      "ghcr.io/kyverno/kyverno",
		want:       true,
	}, {
		name:       "not match",
		expression: `ref != "ghcr.io/kyverno/kyverno"`,
		image:      "ghcr.io/kyverno/kyverno",
		want:       false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewMatchImageEnv()
			assert.NoError(t, err)
			match, errs := CompileMatchImageReference(nil, env, v1alpha1.MatchImageReference{
				Expression: tt.expression,
			})
			assert.Nil(t, errs)
			got, err := match.Match(tt.image)
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
