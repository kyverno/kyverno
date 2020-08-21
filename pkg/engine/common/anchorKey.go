package common

// AnchorKey - contains map of anchors
type AnchorKey struct {
	// anchorMap - for each anchor key in the patterns it will maintains information if the key exists in the resource
	// if anchor key of the pattern exists in the resource then (key)=true else (key)=false
	anchorMap map[string]bool
	// AnchorError - used in validate to break execution of the recursion when if condition fails
	AnchorError error
}
// NewAnchorMap -initialize anchorMap
func NewAnchorMap() *AnchorKey {
	return &AnchorKey{anchorMap: make(map[string]bool)}
}
// IsAnchorError - if any of the anchor key doesn't exists in the resource then it will return true
// if any of (key)=false then return IsAnchorError() as true
// if all the keys exists in the pattern exists in resource then return IsAnchorError() as false
func (ac *AnchorKey) IsAnchorError() bool {
	for _, v := range ac.anchorMap {
		if v == false{
			return true
		}
	}
	return false
}
// CheckAnchorInResource 
// Check if condition anchor key has values
func (ac *AnchorKey) CheckAnchorInResource(pattern interface{}, resource interface{}){
	switch typed := pattern.(type) {
	case map[string]interface{}:
		for key := range typed {
			if isConditionAnchor(key) {
				val, ok := ac.anchorMap[key]
				if !ok {
					ac.anchorMap[key] = false
				} else if ok && val == true {
					continue
				}
				if doesAnchorsKeyHasValue(key,resource) {
					ac.anchorMap[key] = true
				}
			}
		}
	}
}

// Checks if anchor key has value in resource
func doesAnchorsKeyHasValue(key string, pattern interface{}) bool {
	akey := removeAnchor(key)
	switch typed := pattern.(type) {
	case map[string]interface{}:
		if _, ok := typed[akey]; ok {
			return true
		}
		for _, value := range typed {
			doesAnchorsKeyHasValue(key,value)
		}
		return false
	case []interface{}:
		for _, value := range typed {
			doesAnchorsKeyHasValue(key, value)
		}
		return false
	default:
		return false
	}
}

func removeAnchor(key string) string {
	if isConditionAnchor(key) {
		return key[1 : len(key)-1]
	}

	if isExistenceAnchor(key) || isEqualityAnchor(key) || isNegationAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}

//IsConditionAnchor checks for condition anchor
func isConditionAnchor(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}

//IsNegationAnchor checks for negation anchor
func isNegationAnchor(str string) bool {
	left := "X("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

// IsEqualityAnchor checks for equality anchor
func isEqualityAnchor(str string) bool {
	left := "=("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

//IsExistenceAnchor checks for existence anchor
func isExistenceAnchor(str string) bool {
	left := "^("
	right := ")"

	if len(str) < len(left)+len(right) {
		return false
	}

	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}