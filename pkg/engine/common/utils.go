package common

import (
	"encoding/json"
	"fmt"
	"reflect"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// CopyMap creates a full copy of the target map
func CopyMap(m map[string]interface{}) map[string]interface{} {
	mapCopy := make(map[string]interface{})
	for k, v := range m {
		mapCopy[k] = v
	}

	return mapCopy
}

// CopySlice creates a full copy of the target slice
func CopySlice(s []interface{}) []interface{} {
	sliceCopy := make([]interface{}, len(s))
	copy(sliceCopy, s)

	return sliceCopy
}

// CopySliceOfMaps creates a full copy of the target slice
func CopySliceOfMaps(s []map[string]interface{}) []interface{} {
	sliceCopy := make([]interface{}, len(s))
	for i, v := range s {
		sliceCopy[i] = CopyMap(v)
	}

	return sliceCopy
}

func ToMap(data interface{}) (map[string]interface{}, error) {
	if m, ok := data.(map[string]interface{}); ok {
		return m, nil
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]interface{})
	err = json.Unmarshal(b, &mapData)
	if err != nil {
		return nil, err
	}

	return mapData, nil
}

func GetRawKeyIfWrappedWithAttributes(str string) string {
	if len(str) < 2 {
		return str
	}

	if str[0] == '(' && str[len(str)-1] == ')' {
		return str[1 : len(str)-1]
	} else if (str[0] == '$' || str[0] == '^' || str[0] == '+' || str[0] == '=') && (str[1] == '(' && str[len(str)-1] == ')') {
		return str[2 : len(str)-1]
	} else {
		return str
	}
}

func TransformConditions(original apiextensions.JSON) (interface{}, error) {
	// conditions are currently in the form of []interface{}
	oldConditions, err := utils.ApiextensionsJsonToKyvernoConditions(original)
	if err != nil {
		return nil, err
	}
	switch typedValue := oldConditions.(type) {
	case kyverno.AnyAllConditions:
		return copyAnyAllConditions(typedValue), nil
	case []kyverno.Condition: // backwards compatibility
		return copyOldConditions(typedValue), nil
	}

	return nil, fmt.Errorf("invalid preconditions")
}

func copyAnyAllConditions(original kyverno.AnyAllConditions) kyverno.AnyAllConditions {
	if reflect.DeepEqual(original, kyverno.AnyAllConditions{}) {
		return kyverno.AnyAllConditions{}
	}
	return *original.DeepCopy()
}

// backwards compatibility
func copyOldConditions(original []kyverno.Condition) []kyverno.Condition {
	if len(original) == 0 {
		return []kyverno.Condition{}
	}

	var copies []kyverno.Condition
	for _, condition := range original {
		copies = append(copies, *condition.DeepCopy())
	}

	return copies
}
