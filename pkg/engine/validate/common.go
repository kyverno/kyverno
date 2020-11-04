package validate

import (
	"fmt"
	"strconv"
)

// convertNumberToString converts value to string
func convertNumberToString(value interface{}) (string, error) {
	if value == nil {
		return "0", nil
	}

	switch typed := value.(type) {
	case string:
		return string(typed), nil
	case float64:
		return fmt.Sprintf("%f", typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	case nil:
		return "", fmt.Errorf("got empty string, expect %v", value)
	default:
		return "", fmt.Errorf("could not convert %v to string", typed)
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
