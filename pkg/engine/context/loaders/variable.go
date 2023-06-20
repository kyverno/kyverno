package loaders

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type variableLoader struct {
	logger    logr.Logger
	entry     kyvernov1.ContextEntry
	enginectx enginecontext.Interface
	jp        jmespath.Interface
	data      []byte
}

func NewVariableLoader(
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	jp jmespath.Interface,
) enginecontext.Loader {
	return &variableLoader{
		logger:    logger,
		entry:     entry,
		enginectx: enginectx,
		jp:        jp,
	}
}

func (vl *variableLoader) HasLoaded() bool {
	return vl.data != nil
}

func (vl *variableLoader) LoadData() error {
	return vl.loadVariable()
}

func (vl *variableLoader) loadVariable() (err error) {
	logger := vl.logger
	ctx := vl.enginectx
	entry := vl.entry

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
			variable, err := applyJMESPath(vl.jp, path, variable)
			if err == nil {
				output = variable
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %v: %v", path, variable, err)
			}
		} else {
			output = variable
		}
	} else {
		if path != "" {
			if variable, err := ctx.Query(path); err == nil {
				if variable != nil {
					output = variable
				}
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %v: %v", path, variable, err)
			}
		}
	}

	logger.V(4).Info("evaluated output", "variable name", entry.Name, "output", output)
	if output == nil {
		return fmt.Errorf("failed to add context entry for variable %s since it evaluated to nil", entry.Name)
	}

	vl.data, err = json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to add context entry for variable %s: %v", entry.Name, err)
	}

	return ctx.ReplaceContextEntry(entry.Name, vl.data)
}

func applyJMESPath(jp jmespath.Interface, query string, data interface{}) (interface{}, error) {
	q, err := jp.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", query, err)
	}
	return q.Search(data)
}
