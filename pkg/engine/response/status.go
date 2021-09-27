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

func (s *RuleStatus) String() string {
	return toString[*s]
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

func getRuleStatus(s string) (*RuleStatus, error){
	for k, v := range toID {
		if s == k {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("invalid status: %s", s)
}

func (v *RuleStatus) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	statusVal, err := getRuleStatus(s)
	if err != nil {
		return err
	}

	*v = *statusVal
	return nil
}
