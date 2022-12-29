package policy

// Condition defines condition type
type Condition int

const (
	// NotEvaluate to not evaluate condition
	NotEvaluate Condition = 0
	// Process to evaluate condition
	Process Condition = 1
	// Skip to ignore/skip the condition
	Skip Condition = 2
)
