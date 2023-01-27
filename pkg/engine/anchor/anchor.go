package anchor

import (
	"path"
	"regexp"
	"strings"
)

type AnchorType string

const (
	Condition       AnchorType = ""
	Global          AnchorType = "<"
	Negation        AnchorType = "X"
	AddIfNotPresent AnchorType = "+"
	Equality        AnchorType = "="
	Existence       AnchorType = "^"
)

var regex = regexp.MustCompile(`^(?P<modifier>[+<=X^])?\((?P<key>.+)\)$`)

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
	modifier AnchorType
	key      string
}

// Parse parses a string, returns nil if not an anchor
func Parse(str string) Anchor {
	str = strings.TrimSpace(str)
	values := regex.FindStringSubmatch(str)
	if len(values) == 0 {
		return nil
	}
	return New(AnchorType(values[1]), values[2])
}

// New creates an anchor
func New(modifier AnchorType, key string) Anchor {
	if key == "" {
		return nil
	}
	return anchor{
		modifier: modifier,
		key:      key,
	}
}

// String returns the anchor string.
// Will return an empty string if key is empty.
func String(modifier AnchorType, key string) string {
	if key == "" {
		return ""
	}
	return string(modifier) + "(" + key + ")"
}

// ContainsCondition returns true, if anchor is either condition anchor or global condition anchor
func ContainsCondition(ah Anchor) bool {
	return ah != nil && (ah.IsCondition() || ah.IsGlobal())
}

func (ah anchor) Type() AnchorType {
	return ah.modifier
}

func (ah anchor) Key() string {
	return ah.key
}

func (ah anchor) String() string {
	return String(ah.modifier, ah.key)
}

func (ah anchor) IsCondition() bool {
	return ah.modifier == Condition
}

func (ah anchor) IsGlobal() bool {
	return ah.modifier == Global
}

func (ah anchor) IsNegation() bool {
	return ah.modifier == Negation
}

func (ah anchor) IsAddIfNotPresent() bool {
	return ah.modifier == AddIfNotPresent
}

func (ah anchor) IsEquality() bool {
	return ah.modifier == Equality
}

func (ah anchor) IsExistence() bool {
	return ah.modifier == Existence
}

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	ah := Parse(key)
	if ah == nil {
		return key, ""
	}
	return ah.Key(), string(ah.Type()) + "("
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
