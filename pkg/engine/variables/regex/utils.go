package regex

import (
	"encoding/json"
	"fmt"
)

// IsVariable returns true if the element contains a 'valid' variable {{}}
func IsVariable(value string) bool {
	groups := RegexVariables.FindAllStringSubmatch(value, -1)
	return len(groups) != 0
}

// IsReference returns true if the element contains a 'valid' reference $()
func IsReference(value string) bool {
	groups := RegexReferences.FindAllStringSubmatch(value, -1)
	return len(groups) != 0
}

func ObjectHasVariables(object interface{}) error {
	var err error
	objectJSON, err := json.Marshal(object)
	if err != nil {
		return err
	}
	if len(RegexVariables.FindAllStringSubmatch(string(objectJSON), -1)) > 0 {
		return fmt.Errorf("variables are not allowed")
	}
	return nil
}
