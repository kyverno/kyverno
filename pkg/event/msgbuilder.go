package event

import (
	"fmt"
	"regexp"
)

//MsgKey is an identified to determine the preset message formats
type MsgKey int

//Message id for pre-defined messages
const (
	FPolicyApplyBlockCreate MsgKey = iota
	FPolicyApplyBlockUpdate
	FPolicyBlockResourceUpdate
	FPolicyApplyFailed
	FResourcePolicyFailed
)

func (k MsgKey) String() string {
	return [...]string{
		"Resource %s creation blocked by rule(s) %s",
		"Rule(s) '%s' of policy '%s' blocked update of the resource",
		"Resource %s update blocked by rule(s) %s",
		"Rule(s) '%s' failed to apply on resource %s",
		"Rule(s) '%s' of policy '%s' failed to apply on the resource",
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
