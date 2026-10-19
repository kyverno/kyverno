package apply

import (
	"bytes"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidatePolicies(t *testing.T) {
	deprecatedPolicy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deprecated",
			Namespace: "test-ns",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: "enforce", // lowercase is deprecated
		},
	}

	invalidClusterPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-cluster-policy",
			Namespace: "not-allowed-on-cluster-policy",
		},
	}

	validPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "rule-1",
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
					Validation: &kyvernov1.Validation{
						Message: "label 'app.kubernetes.io/name' is required",
						RawPattern: &apiextensionsv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"app.kubernetes.io/name":"?*"}}}`),
						},
					},
				},
			},
		},
	}

	t.Run("empty policies", func(t *testing.T) {
		var out bytes.Buffer
		c := &ApplyCommandConfig{}
		var skipped SkippedInvalidPolicies
		var rc processor.ResultCounts

		valid := c.validatePolicies(&out, nil, &skipped, &rc)
		assert.Empty(t, valid)
		assert.Empty(t, skipped.skipped)
		assert.Empty(t, skipped.invalid)
		assert.Empty(t, out.String())
		assert.Empty(t, c.deprecationWarnings)
		assert.NoError(t, c.failOnDeprecationWarnings())
	})

	t.Run("policy with deprecation warning without warnings-as-errors", func(t *testing.T) {
		var out bytes.Buffer
		c := &ApplyCommandConfig{
			warningsAsErrors: false,
		}
		var skipped SkippedInvalidPolicies
		var rc processor.ResultCounts

		valid := c.validatePolicies(&out, []kyvernov1.PolicyInterface{deprecatedPolicy}, &skipped, &rc)
		assert.Len(t, valid, 1)
		assert.Empty(t, skipped.skipped)
		assert.Empty(t, skipped.invalid)
		assert.Contains(t, out.String(), "Warning: policy test-ns/test-deprecated:")
		assert.Contains(t, out.String(), "Validation failure actions enforce/audit are deprecated")
		assert.Len(t, c.deprecationWarnings, 1)
		assert.NoError(t, c.failOnDeprecationWarnings())
	})

	t.Run("policy with deprecation warning with warnings-as-errors", func(t *testing.T) {
		var out bytes.Buffer
		c := &ApplyCommandConfig{
			warningsAsErrors: true,
		}
		var skipped SkippedInvalidPolicies
		var rc processor.ResultCounts

		valid := c.validatePolicies(&out, []kyvernov1.PolicyInterface{deprecatedPolicy}, &skipped, &rc)
		assert.Len(t, valid, 1)
		assert.Empty(t, skipped.skipped)
		assert.Empty(t, skipped.invalid)
		assert.Contains(t, out.String(), "Warning: policy test-ns/test-deprecated:")
		assert.Contains(t, out.String(), "Validation failure actions enforce/audit are deprecated")
		assert.Len(t, c.deprecationWarnings, 1)

		err := c.failOnDeprecationWarnings()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "found 1 deprecation warning(s) and --warnings-as-errors is set")
	})

	t.Run("invalid cluster policy is skipped and duplicate warning suppressed", func(t *testing.T) {
		var out bytes.Buffer
		c := &ApplyCommandConfig{}
		var skipped SkippedInvalidPolicies
		var rc processor.ResultCounts

		valid := c.validatePolicies(&out, []kyvernov1.PolicyInterface{invalidClusterPolicy}, &skipped, &rc)
		assert.Empty(t, valid)
		assert.Len(t, skipped.skipped, 1)
		assert.Equal(t, "invalid-cluster-policy", skipped.skipped[0].name)
		assert.Contains(t, skipped.skipped[0].reason, "A clusterpolicy should not have the namespace defined")
		assert.Empty(t, skipped.invalid)
		assert.Empty(t, out.String())
		assert.Empty(t, c.deprecationWarnings)
		assert.NoError(t, c.failOnDeprecationWarnings())
	})

	t.Run("valid policy with no warnings", func(t *testing.T) {
		var out bytes.Buffer
		c := &ApplyCommandConfig{
			warningsAsErrors: true,
		}
		var skipped SkippedInvalidPolicies
		var rc processor.ResultCounts

		valid := c.validatePolicies(&out, []kyvernov1.PolicyInterface{validPolicy}, &skipped, &rc)
		assert.Len(t, valid, 1)
		assert.Empty(t, skipped.skipped)
		assert.Empty(t, skipped.invalid)
		assert.Empty(t, out.String())
		assert.Empty(t, c.deprecationWarnings)
		assert.NoError(t, c.failOnDeprecationWarnings())
	})
}
