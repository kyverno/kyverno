package assert

import (
	"fmt"
	"reflect"
)

func getKind(value any) reflect.Kind {
	if value == nil {
		return reflect.Invalid
	}
	return reflect.TypeOf(value).Kind()
}

func match(expected, actual any) (bool, error) {
	if expected != nil {
		switch getKind(expected) {
		case reflect.Slice:
			if getKind(actual) != reflect.Slice {
				return false, fmt.Errorf("invalid actual value, must be a slice, found %s", getKind(actual))
			}
			if reflect.ValueOf(expected).Len() != reflect.ValueOf(actual).Len() {
				return false, nil
			}
			for i := 0; i < reflect.ValueOf(expected).Len(); i++ {
				if inner, err := match(reflect.ValueOf(expected).Index(i).Interface(), reflect.ValueOf(actual).Index(i).Interface()); err != nil {
					return false, err
				} else if !inner {
					return false, nil
				}
			}
			return true, nil
		case reflect.Map:
			if getKind(actual) != reflect.Map {
				return false, fmt.Errorf("invalid actual value, must be a map, found %s", getKind(actual))
			}
			iter := reflect.ValueOf(expected).MapRange()
			for iter.Next() {
				actualValue := reflect.ValueOf(actual).MapIndex(iter.Key())
				if !actualValue.IsValid() {
					return false, nil
				}
				if inner, err := match(iter.Value().Interface(), actualValue.Interface()); err != nil {
					return false, err
				} else if !inner {
					return false, nil
				}
			}
			return true, nil
		}
	}
	return matchScalar(expected, actual)
}

func matchScalar(expected, actual any) (bool, error) {
	if actual == nil && expected == nil {
		return true, nil
	} else if actual == nil && expected != nil {
		return false, nil
	} else if actual != nil && expected == nil {
		return false, nil
	}
	if actual == expected {
		return true, nil
	}
	// if they are the same type we can use reflect.DeepEqual
	if reflect.TypeOf(expected) == reflect.TypeOf(actual) {
		return reflect.DeepEqual(expected, actual), nil
	}
	e := reflect.ValueOf(expected)
	a := reflect.ValueOf(actual)
	if !a.IsValid() && !e.IsValid() {
		return true, nil
	}
	// named string types (e.g. rule types) compare with plain strings
	if a.Kind() == reflect.String && e.Kind() == reflect.String {
		return a.String() == e.String(), nil
	}
	if a.CanComplex() && e.CanComplex() {
		return a.Complex() == e.Complex(), nil
	}
	if a.CanFloat() && e.CanFloat() {
		return a.Float() == e.Float(), nil
	}
	if a.CanInt() && e.CanInt() {
		return a.Int() == e.Int(), nil
	}
	if a.CanUint() && e.CanUint() {
		return a.Uint() == e.Uint(), nil
	}
	if a, ok := toNumber(a); ok {
		if e, ok := toNumber(e); ok {
			return a == e, nil
		}
	}
	return false, fmt.Errorf("types are not comparable, %s - %s", getKind(expected), getKind(actual))
}

func toNumber(value reflect.Value) (float64, bool) {
	if value.CanFloat() {
		return value.Float(), true
	}
	if value.CanInt() {
		return float64(value.Int()), true
	}
	if value.CanUint() {
		return float64(value.Uint()), true
	}
	return 0, false
}
