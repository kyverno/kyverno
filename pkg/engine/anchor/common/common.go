package common

// IsAnchor is a function handler
type IsAnchor func(str string) bool

//IsConditionAnchor checks for condition anchor
func IsConditionAnchor(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
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

// IsAddingAnchor checks for addition anchor
func IsAddingAnchor(key string) bool {
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

// RemoveAnchor remove anchor from the given key. It returns
// the anchor-free tag value and the prefix of the anchor.
func RemoveAnchor(key string) (string, string) {
	if IsConditionAnchor(key) {
		return key[1 : len(key)-1], key[0:1]
	}

	if IsExistenceAnchor(key) || IsAddingAnchor(key) || IsEqualityAnchor(key) || IsNegationAnchor(key) {
		return key[2 : len(key)-1], key[0:2]
	}

	return key, ""
}

// AddAnchor adds an anchor with the supplied prefix.
// The suffix is assumed to be ")".
func AddAnchor(key, anchorPrefix string) string {
	return anchorPrefix + key + ")"
}
