package validate

import (
	"fmt"
	"strconv"

	"github.com/nirmata/kyverno/pkg/engine/operator"
)

func isStringIsReference(str string) bool {
	if len(str) < len(operator.ReferenceSign) {
		return false
	}

	return str[0] == '$' && str[1] == '(' && str[len(str)-1] == ')'
}

// convertToFloat converts string and any other value to float64
func convertToFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case string:
		var err error
		floatValue, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, err
		}

		return floatValue, nil
	case float64:
		return typed, nil
	case int64:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("Could not convert %T to float64", value)
	}
}

// convertToString converts value to string
func convertToString(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return string(typed), nil
	case float64:
		return fmt.Sprintf("%f", typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	default:
		return "", fmt.Errorf("Could not convert %T to string", value)
	}
}

func getRawKeyIfWrappedWithAttributes(str string) string {
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
