package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKyvernoVersionConstraintAcceptsSpecExample(t *testing.T) {
	_, err := ParseKyvernoVersionConstraint(">=1.20")
	assert.NoError(t, err)
}

func TestParseKyvernoVersionConstraintRejectsGarbage(t *testing.T) {
	_, err := ParseKyvernoVersionConstraint("not a constraint")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a constraint")
	assert.Contains(t, err.Error(), ">=1.20")
}

func TestCheckKyvernoVersionEmptyConstraintIsSatisfied(t *testing.T) {
	c, err := CheckKyvernoVersion("", "1.20.0")
	require.NoError(t, err)
	assert.Equal(t, CompatibilitySatisfied, c)
}

func TestCheckKyvernoVersionSatisfied(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "1.21.0")
	require.NoError(t, err)
	assert.Equal(t, CompatibilitySatisfied, c)
}

func TestCheckKyvernoVersionUnsatisfied(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "1.19.0")
	require.NoError(t, err)
	assert.Equal(t, CompatibilityUnsatisfied, c)
}

func TestCheckKyvernoVersionStripsLeadingV(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "v1.21.0")
	require.NoError(t, err)
	assert.Equal(t, CompatibilitySatisfied, c)
}

func TestCheckKyvernoVersionIgnoresPrerelease(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "1.20.0-rc.1")
	require.NoError(t, err)
	assert.Equal(t, CompatibilitySatisfied, c, "an RC build of the CLI's own release line must not report unsatisfied")
}

func TestCheckKyvernoVersionUnknownForDevBuilds(t *testing.T) {
	for _, v := range []string{"---", "(devel)", "v0.0.0-20231010120000-abcdef123456", ""} {
		t.Run(v, func(t *testing.T) {
			c, err := CheckKyvernoVersion(">=1.20", v)
			require.NoError(t, err)
			assert.Equal(t, CompatibilityUnknown, c)
		})
	}
}

func TestCheckKyvernoVersionMalformedConstraintErrors(t *testing.T) {
	_, err := CheckKyvernoVersion("not a constraint", "1.20.0")
	assert.Error(t, err)
}

func TestDescribeCompatibilitySatisfiedIsSilent(t *testing.T) {
	text, isWarning := DescribeCompatibility(CompatibilitySatisfied, ">=1.20", "1.21.0")
	assert.Empty(t, text)
	assert.False(t, isWarning)
}

func TestDescribeCompatibilityUnsatisfiedIsWarning(t *testing.T) {
	text, isWarning := DescribeCompatibility(CompatibilityUnsatisfied, ">=1.20", "1.19.0")
	assert.True(t, isWarning)
	assert.Contains(t, text, "Warning:")
	assert.Contains(t, text, ">=1.20")
	assert.Contains(t, text, "1.19.0")
}

func TestDescribeCompatibilityUnknownIsNote(t *testing.T) {
	text, isWarning := DescribeCompatibility(CompatibilityUnknown, ">=1.20", "---")
	assert.False(t, isWarning)
	assert.Contains(t, text, "Note:")
}
