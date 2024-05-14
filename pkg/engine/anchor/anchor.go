package anchor

import (
	"regexp"
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
}

type anchor struct {
	modifier AnchorType
	key      string
}

// Parse parses a string, returns nil if not an anchor
func Parse(str string) Anchor {
	if len(str) < 2 {
		return nil
	}

	var modifier AnchorType
	var key string

	if str[len(str)-1] == ')' {
		if str[0] == '(' {
			key = str[1 : len(str)-1]
		} else if str[0] == '+' || str[0] == '=' || str[0] == 'X' || str[0] == '^' || str[0] == '<' {
			modifier = AnchorType(str[0])
			key = str[2 : len(str)-1]
		} else {
			return nil
		}
	} else {
		return nil
	}

	return New(AnchorType(modifier), key)
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

func (a anchor) Type() AnchorType {
	return a.modifier
}

func (a anchor) Key() string {
	return a.key
}

func (a anchor) String() string {
	return String(a.modifier, a.key)
}

// IsOneOf returns checks if anchor is one of the given types
func IsOneOf(a Anchor, types ...AnchorType) bool {
	if a != nil {
		for _, t := range types {
			if t == a.Type() {
				return true
			}
		}
	}
	return false
}

// ContainsCondition returns true if anchor is either condition anchor or global condition anchor
func ContainsCondition(a Anchor) bool {
	return IsOneOf(a, Condition, Global)
}

// IsCondition checks for condition anchor
func IsCondition(a Anchor) bool {
	return IsOneOf(a, Condition)
}

// IsGlobal checks for global condition anchor
func IsGlobal(a Anchor) bool {
	return IsOneOf(a, Global)
}

// IsNegation checks for negation anchor
func IsNegation(a Anchor) bool {
	return IsOneOf(a, Negation)
}

// IsAddIfNotPresent checks for addition anchor
func IsAddIfNotPresent(a Anchor) bool {
	return IsOneOf(a, AddIfNotPresent)
}

// IsEquality checks for equality anchor
func IsEquality(a Anchor) bool {
	return IsOneOf(a, Equality)
}

// IsExistence checks for existence anchor
func IsExistence(a Anchor) bool {
	return IsOneOf(a, Existence)
}
