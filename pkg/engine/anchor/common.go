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

type Anchor interface {
	IsConditionAnchor() bool
	IsGlobalAnchor() bool
	ContainsCondition() bool
	IsNegationAnchor() bool
	IsAddIfNotPresentAnchor() bool
	IsEqualityAnchor() bool
	IsExistenceAnchor() bool
	IsAnchor() bool
	Type() AnchorType
	Key() string
}

type anchor struct {
	modifier string
	key      string
}

func ParseAnchor(str string) Anchor {
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

// IsConditionAnchor checks for condition anchor
func (ah anchor) IsConditionAnchor() bool {
	return ah.modifier == string(ConditionAnchor)
}

// IsGlobalAnchor checks for global condition anchor
func (ah anchor) IsGlobalAnchor() bool {
	return ah.modifier == string(GlobalAnchor)
}

// ContainsCondition returns true, if str is either condition anchor or
// global condition anchor
func (ah anchor) ContainsCondition() bool {
	return ah.IsConditionAnchor() || ah.IsGlobalAnchor()
}

// IsNegationAnchor checks for negation anchor
func (ah anchor) IsNegationAnchor() bool {
	return ah.modifier == string(NegationAnchor)
}

// IsAddIfNotPresentAnchor checks for addition anchor
func (ah anchor) IsAddIfNotPresentAnchor() bool {
	return ah.modifier == string(AddIfNotPresentAnchor)
}

// IsEqualityAnchor checks for equality anchor
func (ah anchor) IsEqualityAnchor() bool {
	return ah.modifier == string(EqualityAnchor)
}

// IsExistenceAnchor checks for existence anchor
func (ah anchor) IsExistenceAnchor() bool {
	return ah.modifier == string(ExistenceAnchor)
}

// IsAnchor checks for existence anchor
func (ah anchor) IsAnchor() bool {
	return ah.key != ""
}

func (ah anchor) Type() AnchorType {
	return AnchorType(ah.key)
}

func (ah anchor) Key() string {
	return ah.key
}

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	ah := ParseAnchor(key)
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
