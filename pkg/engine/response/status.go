package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RuleStatus represents the status of rule execution
type RuleStatus int

// RuleStatusPass is used to report the result of processing a rule.
const (
	// RuleStatusPass indicates that the resources meets the policy rule requirements
	RuleStatusPass RuleStatus = iota
	// Fail indicates that the resource does not meet the policy rule requirements
	RuleStatusFail
	// Warn indicates that the the resource does not meet the policy rule requirements, but the policy is not scored
	RuleStatusWarn
	// Error indicates that the policy rule could not be evaluated due to a processing error, for
	// example when a variable cannot be resolved  in the policy rule definition. Note that variables
	// that cannot be resolved in preconditions are replaced with empty values to allow existence
	// checks.
	RuleStatusError
	// Skip indicates that the policy rule was not selected based on user inputs or applicability, for example
	// when preconditions are not met, or when conditional or global anchors are not satistied.
	RuleStatusSkip
)

func (s *RuleStatus) String() string {
	return toString[*s]
}

var toString = map[RuleStatus]string{
	RuleStatusPass:  "pass",
	RuleStatusFail:  "fail",
	RuleStatusWarn:  "warning",
	RuleStatusError: "error",
	RuleStatusSkip:  "skip",
}

var toID = map[string]RuleStatus{
	"pass":    RuleStatusPass,
	"fail":    RuleStatusFail,
	"warning": RuleStatusWarn,
	"error":   RuleStatusError,
	"skip":    RuleStatusSkip,
}

// MarshalJSON marshals the enum as a quoted json string
func (s *RuleStatus) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "\"%s\"", toString[*s])
	return []byte(b.String()), nil
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (s *RuleStatus) UnmarshalJSON(b []byte) error {
	var strVal string
	err := json.Unmarshal(b, &strVal)
	if err != nil {
		return err
	}

	statusVal, err := getRuleStatus(strVal)
	if err != nil {
		return err
	}

	*s = *statusVal
	return nil
}

func getRuleStatus(s string) (*RuleStatus, error) {
	for k, v := range toID {
		if s == k {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("invalid status: %s", s)
}

func (s *RuleStatus) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	statusVal, err := getRuleStatus(str)
	if err != nil {
		return err
	}

	*s = *statusVal
	return nil
}
