package anchor

import (
	"path"
	"regexp"
	"strings"
)

type AnchorType string

const (
	ConditionAnchor       AnchorType = ""
	GlobalAnchor          AnchorType = "<"
	NegationAnchor        AnchorType = "X"
	AddIfNotPresentAnchor AnchorType = "+"
	EqualityAnchor        AnchorType = "="
	ExistenceAnchor       AnchorType = "^"
)

var regex = regexp.MustCompile(`^(?P<modifier>[+<=X^])?\((?P<key>\w+)\)$`)

// Anchor interface
type Anchor interface {
	// Type returns the anchor type
	Type() AnchorType
	// Key returns the anchor key
	Key() string
	// String returns the anchor string
	String() string
	// IsCondition checks for condition anchor
	IsCondition() bool
	// IsGlobalAnchor checks for global condition anchor
	IsGlobal() bool
	// ContainsCondition returns true, if str is either condition anchor or global condition anchor
	ContainsCondition() bool
	// IsNegationAnchor checks for negation anchor
	IsNegation() bool
	// IsAddIfNotPresentAnchor checks for addition anchor
	IsAddIfNotPresent() bool
	// IsEqualityAnchor checks for equality anchor
	IsEquality() bool
	// IsExistenceAnchor checks for existence anchor
	IsExistence() bool
}

type anchor struct {
	modifier string
	key      string
}

// Parse parses a string, returns nil if not an anchor
func Parse(str string) Anchor {
	str = strings.TrimSpace(str)
	values := regex.FindStringSubmatch(str)
	if len(values) == 0 {
		return nil
	}
	return &anchor{
		modifier: values[1],
		key:      values[2],
	}
}

func (ah anchor) Type() AnchorType {
	return AnchorType(ah.key)
}

func (ah anchor) Key() string {
	return ah.key
}

func (ah anchor) String() string {
	return ah.modifier + "(" + ah.key + ")"
}

func (ah anchor) IsCondition() bool {
	return ah.modifier == string(ConditionAnchor)
}

func (ah anchor) IsGlobal() bool {
	return ah.modifier == string(GlobalAnchor)
}

func (ah anchor) ContainsCondition() bool {
	return ah.IsCondition() || ah.IsGlobal()
}

func (ah anchor) IsNegation() bool {
	return ah.modifier == string(NegationAnchor)
}

func (ah anchor) IsAddIfNotPresent() bool {
	return ah.modifier == string(AddIfNotPresentAnchor)
}

func (ah anchor) IsEquality() bool {
	return ah.modifier == string(EqualityAnchor)
}

func (ah anchor) IsExistence() bool {
	return ah.modifier == string(ExistenceAnchor)
}

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	ah := Parse(key)
	if ah == nil {
		return "", ""
	}
	return ah.Key(), ""
}

// RemoveAnchorsFromPath removes all anchor from path string
func RemoveAnchorsFromPath(str string) string {
	components := strings.Split(str, "/")
	if components[0] == "" {
		components = components[1:]
	}

	for i, component := range components {
		components[i], _ = RemoveAnchor(component)
	}

	newPath := path.Join(components...)
	if path.IsAbs(str) {
		newPath = "/" + newPath
	}
	return newPath
}

// AddAnchor adds an anchor with the supplied prefix.
// The suffix is assumed to be ")".
func AddAnchor(key, anchorPrefix string) string {
	return anchorPrefix + key + ")"
}
