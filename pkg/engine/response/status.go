package response

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RuleStatus represents the status of rule execution
type RuleStatus int

const (
	// RuleStatusPass indicates that the policy rule requirements are met
	RuleStatusPass RuleStatus = iota
	// Fail indicates that the policy rule requirements are not met
	RuleStatusFail
	// Warn indicates that the policy rule requirements are not met, and the policy is not scored
	RuleStatusWarn
	// Error indicates that the policy rule could not be evaluated due to a processing error
	RuleStatusError
	// Skip indicates that the policy rule was not selected based on user inputs or applicability
	RuleStatusSkip
)

func (s RuleStatus) String() string {
	return toString[s]
}

var toString = map[RuleStatus]string{
	RuleStatusPass:  "Pass",
	RuleStatusFail: "Fail",
	RuleStatusWarn: "Warning",
	RuleStatusError: "Error",
	RuleStatusSkip: "Skip",
}

var toID = map[string]RuleStatus{
	"Pass":  RuleStatusPass,
	"Fail":  RuleStatusFail,
	"Warning": RuleStatusWarn,
	"Error": RuleStatusError,
	"Skip": RuleStatusSkip,
}

// MarshalJSON marshals the enum as a quoted json string
func (s RuleStatus) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "\"%s\"", toString[s])
	return []byte(b.String()), nil
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (s *RuleStatus) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	for k, v := range toID {
		if j == k {
			*s = v
			return nil
		}
	}

	return fmt.Errorf("invalid status: %s", j)
}
