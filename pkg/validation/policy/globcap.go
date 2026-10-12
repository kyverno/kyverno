package policy

import (
	"fmt"
	"unicode/utf8"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Glob length caps for wildcard-bearing policy string fields. These are
// write-time hygiene: they keep the common admission path clean, but do
// not by themselves bound an already-stored policy -- that bound comes
// from the matcher's own step budget.
//
// The annotation cap is tighter because the value it is compared against
// can be a full-size (256KiB) annotation value; the general cap covers
// name/namespace/subject-shaped globs, whose legitimate longest shapes
// (image references, paths) are well under it.
const (
	maxGlobLength           = 2048
	maxAnnotationGlobLength = 255
)

// ValidateResourceDescriptionGlobLengths caps the length of every
// wildcard-bearing string field in a ResourceDescription. Exported so both
// pkg/validation/policy and pkg/validation/cleanuppolicy (which has its
// own validator and does not otherwise go through this package) can apply
// the same cap.
func ValidateResourceDescriptionGlobLengths(prefix string, rd kyvernov1.ResourceDescription) error {
	if err := ValidateStringGlobLengths(prefix+".kinds", rd.Kinds); err != nil {
		return err
	}
	if rd.Name != "" {
		if err := checkGlobLength(prefix+".name", rd.Name, maxGlobLength); err != nil {
			return err
		}
	}
	for i, n := range rd.Names {
		if err := checkGlobLength(fmt.Sprintf("%s.names[%d]", prefix, i), n, maxGlobLength); err != nil {
			return err
		}
	}
	for i, ns := range rd.Namespaces {
		if err := checkGlobLength(fmt.Sprintf("%s.namespaces[%d]", prefix, i), ns, maxGlobLength); err != nil {
			return err
		}
	}
	for key, value := range rd.Annotations {
		if err := checkGlobLength(prefix+".annotations key", key, maxAnnotationGlobLength); err != nil {
			return err
		}
		if err := checkGlobLength(fmt.Sprintf("%s.annotations[%s]", prefix, key), value, maxAnnotationGlobLength); err != nil {
			return err
		}
	}
	if err := ValidateLabelSelectorGlobLengths(prefix+".selector", rd.Selector); err != nil {
		return err
	}
	if err := ValidateLabelSelectorGlobLengths(prefix+".namespaceSelector", rd.NamespaceSelector); err != nil {
		return err
	}
	return nil
}

// ValidateLabelSelectorGlobLengths caps the length of a label selector's
// matchLabels keys and values, which support the same wildcard characters
// as the rest of ResourceDescription (see wildcards.ExpandInMetadata) and
// so get the same general glob cap. A nil selector is a no-op.
func ValidateLabelSelectorGlobLengths(prefix string, selector *metav1.LabelSelector) error {
	if selector == nil {
		return nil
	}
	for key, value := range selector.MatchLabels {
		if err := checkGlobLength(prefix+".matchLabels key", key, maxGlobLength); err != nil {
			return err
		}
		if err := checkGlobLength(fmt.Sprintf("%s.matchLabels[%s]", prefix, key), value, maxGlobLength); err != nil {
			return err
		}
	}
	return nil
}

// ValidateStringGlobLengths caps the length of every entry in a slice of
// wildcard-bearing strings (for example Kinds, or a PolicyException's
// ruleNames) against the general glob cap. Exported for reuse by callers
// outside pkg/validation/policy that hold their own wildcard-bearing
// string lists.
func ValidateStringGlobLengths(prefix string, values []string) error {
	for i, v := range values {
		if err := checkGlobLength(fmt.Sprintf("%s[%d]", prefix, i), v, maxGlobLength); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUserInfoGlobLengths caps the length of every wildcard-bearing
// string field in a UserInfo (roles, cluster roles, and subject names).
func ValidateUserInfoGlobLengths(prefix string, u kyvernov1.UserInfo) error {
	for i, role := range u.Roles {
		if err := checkGlobLength(fmt.Sprintf("%s.roles[%d]", prefix, i), role, maxGlobLength); err != nil {
			return err
		}
	}
	for i, role := range u.ClusterRoles {
		if err := checkGlobLength(fmt.Sprintf("%s.clusterRoles[%d]", prefix, i), role, maxGlobLength); err != nil {
			return err
		}
	}
	for i, subject := range u.Subjects {
		if err := checkGlobLength(fmt.Sprintf("%s.subjects[%d].name", prefix, i), subject.Name, maxGlobLength); err != nil {
			return err
		}
	}
	return nil
}

func checkGlobLength(field, value string, max int) error {
	if n := utf8.RuneCountInString(value); n > max {
		return fmt.Errorf("%s exceeds the maximum allowed length of %d runes (got %d)", field, max, n)
	}
	return nil
}
