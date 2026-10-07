package admissionpolicy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestValidatingPolicyVAPName(t *testing.T) {
	t.Parallel()
	longNamespace := strings.Repeat("n", 63)
	longName := strings.Repeat("p", 253)

	assert.Equal(t, "vpol-foo", ValidatingPolicyVAPName("", "foo"))
	assert.Equal(t, "nvpol-team-a.foo", ValidatingPolicyVAPName("team-a", "foo"))
	// namespaces cannot contain dots, so splitting differently gives a different name
	assert.NotEqual(t, ValidatingPolicyVAPName("team-a", "foo"), ValidatingPolicyVAPName("team", "a-foo"))

	long := ValidatingPolicyVAPName(longNamespace, longName)
	assert.LessOrEqual(t, len(long+"-binding"), 253)
	assert.True(t, strings.HasPrefix(long, "nvpol-"+longNamespace+"."))
	assert.Equal(t, long, ValidatingPolicyVAPName(longNamespace, longName), "names must be deterministic")
	assert.NotEqual(t, long, ValidatingPolicyVAPName(longNamespace, longName[:252]+"q"), "long names must stay distinct")
	assert.LessOrEqual(t, len(ValidatingPolicyVAPName("", longName)+"-binding"), 253)

	// the API server validates VAP and binding names as DNS subdomains
	for _, tc := range []struct{ namespace, name string }{
		{"team-a", "foo"},
		{longNamespace, "foo"},
		{longNamespace, longName},
		{"", longName},
		{longNamespace, strings.Repeat("a", 63) + "." + strings.Repeat("b", 63)},
	} {
		vapName := ValidatingPolicyVAPName(tc.namespace, tc.name)
		assert.Empty(t, validation.IsDNS1123Subdomain(vapName), "invalid VAP name %q", vapName)
		assert.Empty(t, validation.IsDNS1123Subdomain(vapName+"-binding"), "invalid binding name %q", vapName+"-binding")
	}
}
