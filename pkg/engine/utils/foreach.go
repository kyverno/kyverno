package utils

import (
	"fmt"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func EvaluateList(jmesPath string, ctx enginecontext.EvalInterface) ([]interface{}, error) {
	i, err := ctx.Query(jmesPath)
	if err != nil {
		return nil, err
	}

	l, ok := i.([]interface{})
	if !ok {
		return []interface{}{i}, nil
	}

	return l, nil
}

// InvertedElement inverted the order of element for patchStrategicMerge  policies as kustomize patch revering the order of patch resources.
func InvertedElement(elements []interface{}) {
	for i, j := 0, len(elements)-1; i < j; i, j = i+1, j-1 {
		elements[i], elements[j] = elements[j], elements[i]
	}
}

func AddElementToContext(ctx engineapi.PolicyContext, element interface{}, index, nesting int, elementScope *bool) error {
	data, err := jsonutils.DocumentToUntyped(element)
	if err != nil {
		return err
	}
	if err := ctx.JSONContext().AddElement(data, index, nesting); err != nil {
		return fmt.Errorf("failed to add element (%v) to JSON context: %w", element, err)
	}
	dataMap, ok := data.(map[string]interface{})
	// We set scoped to true by default if the data is a map
	// otherwise we do not do element scoped foreach unless the user
	// has explicitly set it to true
	scoped := ok

	// If the user has explicitly provided an element scope
	// we check if data is a map or not. In case it is not a map and the user
	// has set elementscoped to true, we throw an error.
	// Otherwise we set the value to what is specified by the user.
	if elementScope != nil {
		if *elementScope && !ok {
			return fmt.Errorf("cannot use elementScope=true foreach rules for elements that are not maps, expected type=map got type=%T", data)
		}
		scoped = *elementScope
	}
	if scoped {
		u := unstructured.Unstructured{}
		u.SetUnstructuredContent(dataMap)
		ctx.SetElement(u)
	}
	return nil
}
