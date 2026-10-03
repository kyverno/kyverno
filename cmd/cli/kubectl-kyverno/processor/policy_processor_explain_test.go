package processor

import (
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func policyResponse(name string, applied bool, marker string) celengine.ValidatingPolicyResponse {
	return celengine.ValidatingPolicyResponse{
		Policy: &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}},
		Trace:  &trace.Decision{PolicyName: marker, Scope: trace.ScopeTrace{Applied: applied}},
	}
}

func markers(ds []*trace.Decision) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.PolicyName)
	}
	return out
}

func TestSelectTraces_OnePerPolicy(t *testing.T) {
	t.Run("keeps the primary when no variant applied", func(t *testing.T) {
		got := selectTraces([]celengine.ValidatingPolicyResponse{
			policyResponse("p", false, "primary"),
			policyResponse("p", false, "autogen-cronjobs"),
			policyResponse("p", false, "autogen-deployments"),
		})
		assert.Equal(t, []string{"primary"}, markers(got))
	})

	t.Run("prefers the variant that applied", func(t *testing.T) {
		got := selectTraces([]celengine.ValidatingPolicyResponse{
			policyResponse("p", false, "primary"),
			policyResponse("p", true, "autogen-deployments"),
			policyResponse("p", true, "autogen-cronjobs"),
		})
		assert.Equal(t, []string{"autogen-deployments"}, markers(got), "the first applied variant wins")
	})

	t.Run("keeps distinct policies in order and skips responses without a trace", func(t *testing.T) {
		got := selectTraces([]celengine.ValidatingPolicyResponse{
			policyResponse("a", true, "a"),
			{Policy: &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "untraced"}}},
			policyResponse("b", false, "b"),
			policyResponse("a", true, "a-variant"),
		})
		assert.Equal(t, []string{"a", "b"}, markers(got))
	})
}

func TestExplain_PrintsNothingUnlessEnabled(t *testing.T) {
	responses := []celengine.ValidatingPolicyResponse{policyResponse("p", true, "shown")}

	var off strings.Builder
	(&PolicyProcessor{Out: &off}).explain(responses)
	assert.Empty(t, off.String())

	var on strings.Builder
	(&PolicyProcessor{Out: &on, Explain: true}).explain(responses)
	assert.Contains(t, on.String(), "shown")
}
