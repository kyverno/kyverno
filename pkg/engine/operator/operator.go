package operator

import (
	"regexp"
)

// Operator is string alias that represents selection operators enum
type Operator string

const (
	// Equal stands for ==
	Equal Operator = ""
	// MoreEqual stands for >=
	MoreEqual Operator = ">="
	// LessEqual stands for <=
	LessEqual Operator = "<="
	// NotEqual stands for !
	NotEqual Operator = "!"
	// More stands for >
	More Operator = ">"
	// Less stands for <
	Less Operator = "<"
	// Range stands for -
	Range Operator = "-"
	// Range Operator = ","
)

//ReferenceSign defines the operator for anchor reference
const ReferenceSign Operator = "$()"

// GetOperatorFromStringPattern parses opeartor from pattern
func GetOperatorFromStringPattern(pattern string) Operator {
	if len(pattern) < 2 {
		return Equal
	}

	if pattern[:len(MoreEqual)] == string(MoreEqual) {
		return MoreEqual
	}

	if pattern[:len(LessEqual)] == string(LessEqual) {
		return LessEqual
	}

	if pattern[:len(More)] == string(More) {
		return More
	}

	if pattern[:len(Less)] == string(Less) {
		return Less
	}

	if pattern[:len(NotEqual)] == string(NotEqual) {
		return NotEqual
	}

	/* First Version
	if len(strings.Split(pattern, ",")) == 2 {
		isRange := (pattern[0:1] == "[" || pattern[0:1] == "(") && (pattern[len(pattern)-1:len(pattern)] == "]" || pattern[len(pattern)-1:len(pattern)] == ")")
		if isRange {
			return Range
		}
	}
	*/

	/* Second Version */
	if match, _ := regexp.Match(`^(\d+(\.\d+)?)([^-]*)-(\d+(\.\d+)?)([^-]*)$`, []byte(pattern)); match {
		return Range
	}

	return Equal
}
