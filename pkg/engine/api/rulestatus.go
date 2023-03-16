package api

// RuleStatus represents the status of rule execution
type RuleStatus string

const (
	// RuleStatusPass indicates that the resources meets the policy rule requirements
	RuleStatusPass RuleStatus = "pass"
	// RuleStatusFail indicates that the resource does not meet the policy rule requirements
	RuleStatusFail RuleStatus = "fail"
	// RuleStatusWarn indicates that the resource does not meet the policy rule requirements, but the policy is not scored
	RuleStatusWarn RuleStatus = "warning"
	// RuleStatusError indicates that the policy rule could not be evaluated due to a processing error, for
	// example when a variable cannot be resolved  in the policy rule definition. Note that variables
	// that cannot be resolved in preconditions are replaced with empty values to allow existence
	// checks.
	RuleStatusError RuleStatus = "error"
	// RuleStatusSkip indicates that the policy rule was not selected based on user inputs or applicability, for example
	// when preconditions are not met, or when conditional or global anchors are not satisfied.
	RuleStatusSkip RuleStatus = "skip"
)
