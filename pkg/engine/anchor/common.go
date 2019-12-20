package anchor

// Anchor function type
type IsAnchor func(str string) bool

func IsConditionAnchor(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}

func IsNegationAnchor(str string) bool {
	left := "X("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

func IsAddingAnchor(key string) bool {
	const left = "+("
	const right = ")"

	if len(key) < len(left)+len(right) {
		return false
	}

	return left == key[:len(left)] && right == key[len(key)-len(right):]
}

func IsEqualityAnchor(str string) bool {
	left := "=("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

func IsExistanceAnchor(str string) bool {
	left := "^("
	right := ")"

	if len(str) < len(left)+len(right) {
		return false
	}

	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}


func removeAnchor(key string) string {
	if IsConditionAnchor(key) {
		return key[1 : len(key)-1]
	}

	if IsExistanceAnchor(key) || IsAddingAnchor(key) || IsEqualityAnchor(key) || IsNegationAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}