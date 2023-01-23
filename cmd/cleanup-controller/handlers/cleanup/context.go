package cleanup

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

func loadVariable(logger logr.Logger, entry kyvernov1.ContextEntry, ctx enginecontext.Interface) (err error) {
	path := ""
	if entry.Variable.JMESPath != "" {
		jp, err := variables.SubstituteAll(logger, ctx, entry.Variable.JMESPath)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.JMESPath, err)
		}
		path = jp.(string)
		logger.V(4).Info("evaluated jmespath", "variable name", entry.Name, "jmespath", path)
	}
	var defaultValue interface{} = nil
	if entry.Variable.Default != nil {
		value, err := variables.DocumentToUntyped(entry.Variable.Default)
		if err != nil {
			return fmt.Errorf("invalid default for variable %s", entry.Name)
		}
		defaultValue, err = variables.SubstituteAll(logger, ctx, value)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.Default, err)
		}
		logger.V(4).Info("evaluated default value", "variable name", entry.Name, "jmespath", defaultValue)
	}
	var output interface{} = defaultValue
	if entry.Variable.Value != nil {
		value, _ := variables.DocumentToUntyped(entry.Variable.Value)
		variable, err := variables.SubstituteAll(logger, ctx, value)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.Value, err)
		}
		if path != "" {
			variable, err := engine.ApplyJMESPath(path, variable)
			if err == nil {
				output = variable
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %s: %v", path, entry.Variable.Value, err)
			}
		} else {
			output = variable
		}
	} else {
		if path != "" {
			if variable, err := ctx.Query(path); err == nil {
				output = variable
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %v", path, err)
			}
		}
	}
	logger.V(4).Info("evaluated output", "variable name", entry.Name, "output", output)
	if output == nil {
		return fmt.Errorf("unable to add context entry for variable %s since it evaluated to nil", entry.Name)
	}
	if outputBytes, err := json.Marshal(output); err == nil {
		return ctx.ReplaceContextEntry(entry.Name, outputBytes)
	} else {
		return fmt.Errorf("unable to add context entry for variable %s: %w", entry.Name, err)
	}
}
