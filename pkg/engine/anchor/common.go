package anchor

import (
	"errors"
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
}
type anchor struct {
	modifier string
	key      string
}

func ParseAnchor(str string) *anchor {
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
func (ah *anchor) IsConditionAnchor() bool {
	if ah != nil && ah.modifier == string(ConditionAnchor) {
		return true
	}
	return false
}

// IsGlobalAnchor checks for global condition anchor
func (ah *anchor) IsGlobalAnchor() bool {
	if ah != nil && ah.modifier == string(GlobalAnchor) {
		return true
	}
	return false
}

// ContainsCondition returns true, if str is either condition anchor or
// global condition anchor
func (ah *anchor) ContainsCondition() bool {
	return ah.IsConditionAnchor() || ah.IsGlobalAnchor()
}

// IsNegationAnchor checks for negation anchor
func (ah *anchor) IsNegationAnchor() bool {
	if ah != nil && ah.modifier == string(NegationAnchor) {
		return true
	}
	return false
}

// IsAddIfNotPresentAnchor checks for addition anchor
func (ah *anchor) IsAddIfNotPresentAnchor() bool {
	if ah != nil && ah.modifier == string(AddIfNotPresentAnchor) {
		return true
	}
	return false
}

// IsEqualityAnchor checks for equality anchor
func (ah *anchor) IsEqualityAnchor() bool {
	if ah != nil && ah.modifier == string(EqualityAnchor) {
		return true
	}
	return false
}

// IsExistenceAnchor checks for existence anchor
func (ah *anchor) IsExistenceAnchor() bool {
	if ah != nil && ah.modifier == string(ExistenceAnchor) {
		return true
	}
	return false
}

func (ah *anchor) IsAnchor(key AnchorType) bool {
	return ah != nil && ah.key == string(key)
}

func (ah *anchor) Type() (AnchorType, error) {
	if ah == nil {
		return ConditionAnchor, errors.New("invalid string")
	}
	return AnchorType(ah.key), nil
}

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	ah := ParseAnchor(key)
	if ah == nil {
		return "", ""
	}
	return ah.key, ""
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
