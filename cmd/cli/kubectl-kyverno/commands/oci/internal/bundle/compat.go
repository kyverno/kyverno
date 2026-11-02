package bundle

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Compatibility is the three-state result of comparing a bundle's declared kyvernoVersion
// constraint against a Kyverno version.
type Compatibility int

const (
	// CompatibilityUnknown means version is not a release version, so nothing was compared.
	CompatibilityUnknown Compatibility = iota
	CompatibilitySatisfied
	CompatibilityUnsatisfied
)

// ParseKyvernoVersionConstraint wraps semver.NewConstraint with an actionable error naming the
// spec's own example, since the spec's dialect (partial versions, ^, ~, x wildcards, ranges like
// ">=1.20") is wider than strict semver.
func ParseKyvernoVersionConstraint(s string) (*semver.Constraints, error) {
	c, err := semver.NewConstraint(s)
	if err != nil {
		return nil, fmt.Errorf("kyvernoVersion %q is not a valid semver constraint (for example \">=1.20\"): %v", s, err)
	}
	return c, nil
}

// NormalizeKyvernoVersion strips a leading "v" and any prerelease/build metadata, so a release
// candidate build of the engine (1.20.0-rc.1) checks against the same release line as its final
// release (1.20.0) would. It returns ("", false) when version doesn't parse as semver at all, or
// when it normalizes to 0.0.0 -- the value pkg/version.Version() returns for a Go pseudo-version,
// "(devel)", or its own "---" placeholder, none of which name a release. Exported so
// `kyverno oci validate --kyverno-version` can reject a target that isn't a release version using
// the same rule CheckKyvernoVersion uses for Unknown.
func NormalizeKyvernoVersion(version string) (string, bool) {
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return "", false
	}
	core := fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch())
	if core == "0.0.0" {
		return "", false
	}
	return core, true
}

// CheckKyvernoVersion compares version against constraint. An empty constraint is Satisfied
// (nothing was declared). A malformed constraint is an error -- resolveDescriptor already rejects
// one at write time, so this should not occur for a bundle this CLI wrote, but a caller comparing
// against a hand-supplied --kyverno-version value needs the error. version is normalized per
// NormalizeKyvernoVersion; a version that fails to normalize returns Unknown rather than an error,
// since "I can't tell" is not "I checked and it's wrong".
func CheckKyvernoVersion(constraint, version string) (Compatibility, error) {
	if constraint == "" {
		return CompatibilitySatisfied, nil
	}
	c, err := ParseKyvernoVersionConstraint(constraint)
	if err != nil {
		return CompatibilityUnknown, err
	}
	normalized, ok := NormalizeKyvernoVersion(version)
	if !ok {
		return CompatibilityUnknown, nil
	}
	v, err := semver.NewVersion(normalized)
	if err != nil {
		return CompatibilityUnknown, nil
	}
	if c.Check(v) {
		return CompatibilitySatisfied, nil
	}
	return CompatibilityUnsatisfied, nil
}

// DescribeCompatibility renders c as stderr text for constraint/version, and whether it's a
// warning (true) or an informational note (false). Satisfied has no text: callers print nothing
// for it. It's a free function over explicit strings, not a method that reads version.Version(),
// so tests don't need to mutate the global version.BuildVersion under paralleltest.
func DescribeCompatibility(c Compatibility, constraint, version string) (text string, isWarning bool) {
	switch c {
	case CompatibilitySatisfied:
		return "", false
	case CompatibilityUnsatisfied:
		return fmt.Sprintf("Warning: bundle declares kyvernoVersion %q, which this CLI (%s) does not satisfy; CEL compilation reflects this CLI's engine, not the target's", constraint, version), true
	default:
		return fmt.Sprintf("Note: skipping kyvernoVersion %q check; this CLI's version %q is not a release version", constraint, version), false
	}
}
