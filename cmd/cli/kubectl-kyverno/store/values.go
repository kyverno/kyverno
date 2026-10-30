package store

import (
	"fmt"

	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

// AddValues injects values into the JSON context in two passes: dotted-path variables first,
// then full-object replacements (request.object / request.oldObject). Full-object values replace
// the resource instead of merging into it, and always win over any dotted-path variable that
// targets the same key, regardless of Go map iteration order.
func AddValues(jsonContext enginecontext.Interface, values map[string]interface{}) error {
	for key, value := range values {
		if key == "request.object" || key == "request.oldObject" {
			continue
		}
		if err := jsonContext.AddVariable(key, value); err != nil {
			return fmt.Errorf("failed to add variable to context %s (%w)", key, err)
		}
	}
	for _, key := range []string{"request.object", "request.oldObject"} {
		value, ok := values[key]
		if !ok {
			continue
		}
		var err error
		obj, isMap := value.(map[string]interface{})
		switch {
		case key == "request.object" && isMap:
			err = jsonContext.AddResource(obj)
		case key == "request.oldObject" && isMap:
			err = jsonContext.AddOldResource(obj)
		default:
			err = jsonContext.AddVariable(key, value)
		}
		if err != nil {
			return fmt.Errorf("failed to add variable to context %s (%w)", key, err)
		}
	}
	return nil
}
