package anchor

import (
	"path"
	"regexp"
	"strings"
)

// IsAnchor is a function handler
type IsAnchor func(str string) bool

// IsConditionAnchor checks for condition anchor
func IsConditionAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// IsGlobalAnchor checks for global condition anchor
func IsGlobalAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^<\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// ContainsCondition returns true, if str is either condition anchor or
// global condition anchor
func ContainsCondition(str string) bool {
	return IsConditionAnchor(str) || IsGlobalAnchor(str)
}

// IsNegationAnchor checks for negation anchor
func IsNegationAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^X\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// IsAddIfNotPresentAnchor checks for addition anchor
func IsAddIfNotPresentAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^\+\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// IsEqualityAnchor checks for equality anchor
func IsEqualityAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^=\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// IsExistenceAnchor checks for existence anchor
func IsExistenceAnchor(str string) bool {
	str = strings.TrimSpace(str)
	match, err := regexp.MatchString(`^\^\((.*)\)$`, str)
	if err != nil {
		return false
	}
	return match
}

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	if IsConditionAnchor(key) {
		return key[1 : len(key)-1], key[0:1]
	}

	if IsExistenceAnchor(key) || IsAddIfNotPresentAnchor(key) || IsEqualityAnchor(key) || IsNegationAnchor(key) || IsGlobalAnchor(key) {
		return key[2 : len(key)-1], key[0:2]
	}

	return key, ""
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
