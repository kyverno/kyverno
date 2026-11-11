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

// TestCheckKyvernoVersionUnknownForPseudoVersionWithReleaseBase pins the case the base-0.0.0 check
// alone misses: a Go pseudo-version whose embedded base is a real release line (as happens for a
// build off a tag plus commits) must still report Unknown, not that release.
func TestCheckKyvernoVersionUnknownForPseudoVersionWithReleaseBase(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "v1.20.0-0.20261001123456-abcdef123456")
	require.NoError(t, err)
	assert.Equal(t, CompatibilityUnknown, c)
}

// TestCheckKyvernoVersionUnknownForPseudoVersionWithBuildMetadata pins the "+dirty" build-metadata
// case a hand-rolled "ends with +incompatible or nothing" regex would miss.
func TestCheckKyvernoVersionUnknownForPseudoVersionWithBuildMetadata(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "v1.20.0-0.20261001123456-abcdef123456+dirty")
	require.NoError(t, err)
	assert.Equal(t, CompatibilityUnknown, c)
}

// TestCheckKyvernoVersionUnknownForPseudoVersionWithoutDottedBase pins the no-base pseudo-version
// form, used when there's no earlier tag on the module's major version: no "0." (or "pre.0.")
// segment precedes the timestamp.
func TestCheckKyvernoVersionUnknownForPseudoVersionWithoutDottedBase(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.0", "v1.0.0-20261001123456-abcdef123456")
	require.NoError(t, err)
	assert.Equal(t, CompatibilityUnknown, c)
}

// TestCheckKyvernoVersionIgnoresPrereleaseResemblingPseudoVersionTimestamp pins the inverse of the
// above: a prerelease tag that happens to contain a 14-digit, dash, 12-hex-digit substring (for
// example because someone embedded a date in an RC label) is not a Go pseudo-version and must
// still normalize to its release line like any other prerelease.
func TestCheckKyvernoVersionIgnoresPrereleaseResemblingPseudoVersionTimestamp(t *testing.T) {
	c, err := CheckKyvernoVersion(">=1.20", "1.20.0-rc.20261001123456-abcdef123456")
	require.NoError(t, err)
	assert.Equal(t, CompatibilitySatisfied, c, "an RC label is not a pseudo-version even if it contains a timestamp-shaped substring")
}

// TestNormalizeKyvernoVersionPseudoVersionBothVSpellings pins that NormalizeKyvernoVersion treats
// a pseudo-version identically whether or not the caller's string already has the "v" prefix --
// pkg/version.Version() and descriptor values reach this function with either spelling.
func TestNormalizeKyvernoVersionPseudoVersionBothVSpellings(t *testing.T) {
	withV := "v1.20.0-0.20261001123456-abcdef123456"
	withoutV := "1.20.0-0.20261001123456-abcdef123456"

	_, okWithV := NormalizeKyvernoVersion(withV)
	_, okWithoutV := NormalizeKyvernoVersion(withoutV)
	assert.False(t, okWithV)
	assert.False(t, okWithoutV)
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
