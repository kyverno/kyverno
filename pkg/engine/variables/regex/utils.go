package regex

import (
	"encoding/json"
	"fmt"
)

// IsVariable returns true if the element contains a 'valid' variable {{}}
func IsVariable(value string) bool {
	groups := RegexVariables.FindStringSubmatchIndex(value)
	return len(groups) != 0
}

// IsReference returns true if the element contains a 'valid' reference $()
func IsReference(value string) bool {
	groups := RegexReferences.FindStringSubmatchIndex(value)
	return len(groups) != 0
}

func ObjectHasVariables(object interface{}) error {
	var err error
	objectJSON, err := json.Marshal(object)
	if err != nil {
		return err
	}
	if len(RegexVariables.FindStringSubmatchIndex(string(objectJSON))) > 0 {
		return fmt.Errorf("variables are not allowed")
	}
	return nil
}
