package common

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

func TransformConditions(original apiextensions.JSON) (interface{}, error) {
	// conditions are currently in the form of []interface{}
	oldConditions, err := apiutils.ApiextensionsJsonToKyvernoConditions(original)
	if err != nil {
		return nil, err
	}
	switch typedValue := oldConditions.(type) {
	case kyvernov1.AnyAllConditions:
		return *typedValue.DeepCopy(), nil
	case []kyvernov1.Condition: // backwards compatibility
		var copies []kyvernov1.Condition
		for _, condition := range typedValue {
			copies = append(copies, *condition.DeepCopy())
		}
		return copies, nil
	}
	return nil, fmt.Errorf("invalid preconditions")
}
