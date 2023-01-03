package common

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

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
