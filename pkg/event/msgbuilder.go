package event

import (
	"fmt"
	"regexp"
)

//MsgKey is an identified to determine the preset message formats
type MsgKey int

//Message id for pre-defined messages
const (
	FResourcePolcy MsgKey = iota
	FProcessRule
	SPolicyApply
	SRuleApply
	FPolicyApplyBlockCreate
	FPolicyApplyBlockUpdate
	FPolicyApplyBlockUpdateRule
)

func (k MsgKey) String() string {
	return [...]string{
		"Failed to satisfy policy on resource '%s'.The following rules '%s' failed to apply. Created Policy Violation",
		"Failed to process rule '%s' of policy '%s'. Created Policy Violation",
		"Policy applied successfully on the resource '%s'",
		"Rules '%s' of Policy '%s' applied successful",
		"Failed to apply policy, blocked creation of resource '%s'. The following rules '%s' failed to apply",
		"Failed to apply rule '%s' of policy '%s' Blocked update of the resource",
		"Failed to apply policy on resource '%s'.Blocked update of the resource. The following rules '%s' failed to apply",
	}[k]
}

const argRegex = "%[s,d,v]"

var re = regexp.MustCompile(argRegex)

//GetEventMsg return the application message based on the message id and the arguments,
// if the number of arguments passed to the message are incorrect generate an error
func getEventMsg(key MsgKey, args ...interface{}) (string, error) {
	// Verify the number of arguments
	argsCount := len(re.FindAllString(key.String(), -1))
	if argsCount != len(args) {
		return "", fmt.Errorf("message expects %d arguments, but %d arguments passed", argsCount, len(args))
	}
	return fmt.Sprintf(key.String(), args...), nil
}
