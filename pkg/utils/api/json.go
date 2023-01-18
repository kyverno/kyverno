package api

import (
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// Deserialize "apiextensions.JSON" to a typed array
func DeserializeJSONArray[T any](in apiextensions.JSON) ([]T, error) {
	if in == nil {
		return nil, nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var res []T
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// ApiextensionsJsonToKyvernoConditions takes in user-provided conditions in abstract apiextensions.JSON form
// and converts it into []kyverno.Condition or kyverno.AnyAllConditions according to its content.
// it also helps in validating the condtions as it returns an error when the conditions are provided wrongfully by the user.
func ApiextensionsJsonToKyvernoConditions(in apiextensions.JSON) (interface{}, error) {
	path := "preconditions/validate.deny.conditions"

	// checks for the existence any other field apart from 'any'/'all' under preconditions/validate.deny.conditions
	unknownFieldChecker := func(jsonByteArr []byte, path string) error {
		allowedKeys := map[string]bool{
			"any": true,
			"all": true,
		}
		var jsonDecoded map[string]interface{}
		if err := json.Unmarshal(jsonByteArr, &jsonDecoded); err != nil {
			return fmt.Errorf("error occurred while checking for unknown fields under %s: %+v", path, err)
		}
		for k := range jsonDecoded {
			if !allowedKeys[k] {
				return fmt.Errorf("unknown field '%s' found under %s", k, path)
			}
		}
		return nil
	}

	// marshalling the abstract apiextensions.JSON back to JSON form
	jsonByte, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("error occurred while marshalling %s: %+v", path, err)
	}

	var kyvernoOldConditions []kyvernov1.Condition
	if err = json.Unmarshal(jsonByte, &kyvernoOldConditions); err == nil {
		var validConditionOperator bool

		for _, jsonOp := range kyvernoOldConditions {
			for _, validOp := range kyvernov1.ConditionOperators {
				if jsonOp.Operator == validOp {
					validConditionOperator = true
				}
			}
			if !validConditionOperator {
				return nil, fmt.Errorf("invalid condition operator: %s", jsonOp.Operator)
			}
			validConditionOperator = false
		}

		return kyvernoOldConditions, nil
	}

	var kyvernoAnyAllConditions kyvernov1.AnyAllConditions
	if err = json.Unmarshal(jsonByte, &kyvernoAnyAllConditions); err == nil {
		// checking if unknown fields exist or not
		err = unknownFieldChecker(jsonByte, path)
		if err != nil {
			return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
		}
		return kyvernoAnyAllConditions, nil
	}
	return nil, fmt.Errorf("error occurred while parsing %s: %+v", path, err)
}
