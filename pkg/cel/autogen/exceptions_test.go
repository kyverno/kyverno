package autogen

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func newTestException(matchExpr, valExpr, msgExpr string) *policiesv1beta1.PolicyException {
	polex := &policiesv1beta1.PolicyException{}
	if matchExpr != "" {
		polex.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
			{Name: "match", Expression: matchExpr},
		}
	}
	if valExpr != "" || msgExpr != "" {
		polex.Spec.Validations = []admissionregistrationv1.Validation{
			{Expression: valExpr, MessageExpression: msgExpr},
		}
	}
	return polex
}

func TestRewriteExceptions(t *testing.T) {
	tests := []struct {
		name       string
		exceptions []*policiesv1beta1.PolicyException
		config     string
		wantMatch  string
		wantVal    string
		wantMsg    string
	}{
		{
			name:       "deployments containers expression is rewritten",
			exceptions: []*policiesv1beta1.PolicyException{newTestException("object.spec.containers.exists(c, c.name == 'nginx')", "", "")},
			config:     AutogenDefaults,
			wantMatch:  "object.spec.template.spec.containers.exists(c, c.name == 'nginx')",
		},
		{
			name:       "cronjobs containers expression is rewritten",
			exceptions: []*policiesv1beta1.PolicyException{newTestException("", "object.spec.containers.all(c, c.image.startsWith('ghcr.io/'))", "object.spec.containers[0].name + ' invalid'")},
			config:     AutogenCronjobs,
			wantVal:    "object.spec.jobTemplate.spec.template.spec.containers.all(c, c.image.startsWith('ghcr.io/'))",
			wantMsg:    "object.spec.jobTemplate.spec.template.spec.containers[0].name + ' invalid'",
		},
		{
			name: "metadata path is strictly preserved while spec is rewritten",
			exceptions: []*policiesv1beta1.PolicyException{
				newTestException(
					"object.metadata.name == 'prod' && object.spec.initContainers.size() > 0",
					"",
					"",
				),
			},
			config:    AutogenDefaults,
			wantMatch: "object.metadata.name == 'prod' && object.spec.template.spec.initContainers.size() > 0",
		},
		{
			name:       "empty config returns unchanged",
			exceptions: []*policiesv1beta1.PolicyException{newTestException("object.spec.containers.size() > 0", "", "")},
			config:     "unknown",
			wantMatch:  "object.spec.containers.size() > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RewriteExceptions(tt.exceptions, tt.config)
			require.NoError(t, err)
			require.Len(t, got, len(tt.exceptions))

			if tt.wantMatch != "" {
				assert.Equal(t, tt.wantMatch, got[0].Spec.MatchConditions[0].Expression)
			}
			if tt.wantVal != "" {
				assert.Equal(t, tt.wantVal, got[0].Spec.Validations[0].Expression)
			}
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, got[0].Spec.Validations[0].MessageExpression)
			}
		})
	}
}

func TestRewriteExceptions_Immutability(t *testing.T) {
	origMatch := "object.spec.containers.size() > 0"
	polex := newTestException(origMatch, "", "")
	orig := []*policiesv1beta1.PolicyException{polex}

	got, err := RewriteExceptions(orig, AutogenDefaults)
	require.NoError(t, err)

	assert.Equal(t, origMatch, orig[0].Spec.MatchConditions[0].Expression, "original exception must not be mutated")
	assert.NotEqual(t, orig[0].Spec.MatchConditions[0].Expression, got[0].Spec.MatchConditions[0].Expression)
}

func TestRewriteExceptions_NilInput(t *testing.T) {
	got, err := RewriteExceptions(nil, AutogenDefaults)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = RewriteExceptions([]*policiesv1beta1.PolicyException{}, AutogenDefaults)
	require.NoError(t, err)
	assert.Nil(t, got)
}
