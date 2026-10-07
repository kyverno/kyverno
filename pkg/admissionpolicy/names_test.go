package admissionpolicy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
}
