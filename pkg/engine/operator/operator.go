package operator

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
)

const relativePrefix Operator = "./"
const ReferenceSign Operator = "$()"

// getOperatorFromStringPattern parses opeartor from pattern
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

	return Equal
}
