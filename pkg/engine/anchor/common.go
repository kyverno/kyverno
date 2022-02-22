package anchor

import (
	"path"
	"strings"
)

// IsAnchor is a function handler
type IsAnchor func(str string) bool

//IsConditionAnchor checks for condition anchor
func IsConditionAnchor(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}

//IsGlobalAnchor checks for global condition anchor
func IsGlobalAnchor(str string) bool {
	left := "<("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}

	leftMatch := strings.TrimSpace(str[:len(left)]) == left
	rightMatch := strings.TrimSpace(str[len(str)-len(right):]) == right
	return leftMatch && rightMatch
}

//ContainsCondition returns true, if str is either condition anchor or
// global condition anchor
func ContainsCondition(str string) bool {
	return IsConditionAnchor(str) || IsGlobalAnchor(str)
}

//IsNegationAnchor checks for negation anchor
func IsNegationAnchor(str string) bool {
	left := "X("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

// IsAddIfNotPresentAnchor checks for addition anchor
func IsAddIfNotPresentAnchor(key string) bool {
	const left = "+("
	const right = ")"

	if len(key) < len(left)+len(right) {
		return false
	}

	return left == key[:len(left)] && right == key[len(key)-len(right):]
}

// IsEqualityAnchor checks for equality anchor
func IsEqualityAnchor(str string) bool {
	left := "=("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

//IsExistenceAnchor checks for existence anchor
func IsExistenceAnchor(str string) bool {
	left := "^("
	right := ")"

	if len(str) < len(left)+len(right) {
		return false
	}

	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

// IsNonAnchor checks that key does not have any anchor
func IsNonAnchor(str string) bool {
	key, _ := RemoveAnchor(str)
	return str == key
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
