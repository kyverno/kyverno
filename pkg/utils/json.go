package utils

import (
	"github.com/golang/glog"
)

//JSONsubsetValue checks if JSON a is contained in JSON b
func JSONsubsetValue(a interface{}, b interface{}) bool {
	switch typed := a.(type) {
	case bool:
		bv, ok := b.(bool)
		if !ok {
			glog.Errorf("expected bool found %T", b)
			return false
		}
		av, _ := a.(bool)
		if av == bv {
			return true
		}
	case int:
		bv, ok := b.(int)
		if !ok {
			glog.Errorf("expected int found %T", b)
			return false
		}
		av, _ := a.(int)
		if av == bv {
			return true
		}
	case float64:
		bv, ok := b.(float64)
		if !ok {
			glog.Errorf("expected float64 found %T", b)
			return false
		}
		av, _ := a.(float64)
		if av == bv {
			return true
		}

	case string:
		bv, ok := b.(string)
		if !ok {
			glog.Errorf("expected string found %T", b)
			return false
		}
		av, _ := a.(string)
		if av == bv {
			return true
		}

	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			glog.Errorf("expected map[string]interface{} found %T", b)
			return false
		}
		av, _ := a.(map[string]interface{})
		return subsetMap(av, bv)
	case []interface{}:
		// TODO: verify the logic
		bv, ok := b.([]interface{})
		if !ok {
			glog.Errorf("expected []interface{} found %T", b)
			return false
		}
		av, _ := a.([]interface{})
		return subsetSlice(av, bv)
	default:
		glog.Errorf("Unspported type %s", typed)

	}
	return false
}

func subsetMap(a, b map[string]interface{}) bool {
	// check if keys are present
	for k := range a {
		if _, ok := b[k]; !ok {
			glog.Errorf("key %s, not present in resource", k)
			return false
		}
	}
	// check if values for the keys match
	for ak, av := range a {
		bv := b[ak]
		if !JSONsubsetValue(av, bv) {
			return false
		}
	}
	return true
}

func contains(a interface{}, b []interface{}) bool {
	switch typed := a.(type) {
	case bool:
		for _, bv := range b {
			bv, ok := bv.(bool)
			if !ok {
				return false
			}
			av, _ := a.(bool)

			if bv == av {
				return true
			}
		}
	case int:
		for _, bv := range b {
			bv, ok := bv.(int)
			if !ok {
				return false
			}
			av, _ := a.(int)

			if bv == av {
				return true
			}
		}
	case float64:
		for _, bv := range b {
			bv, ok := bv.(float64)
			if !ok {
				return false
			}
			av, _ := a.(float64)

			if bv == av {
				return true
			}
		}
	case string:
		for _, bv := range b {
			bv, ok := bv.(string)
			if !ok {
				return false
			}
			av, _ := a.(string)

			if bv == av {
				return true
			}
		}
	case map[string]interface{}:
		for _, bv := range b {
			bv, ok := bv.(map[string]interface{})
			if !ok {
				return false
			}
			av, _ := a.(map[string]interface{})
			if subsetMap(av, bv) {
				return true
			}
		}
	case []interface{}:
		for _, bv := range b {
			bv, ok := bv.([]interface{})
			if !ok {
				return false
			}
			av, _ := a.([]interface{})
			if JSONsubsetValue(av, bv) {
				return true
			}
		}
	default:
		glog.Errorf("Unspported type %s", typed)
	}

	return false
}

func subsetSlice(a, b []interface{}) bool {
	// if empty
	if len(a) == 0 {
		return true
	}
	// check if len is not greater
	if len(a) > len(b) {
		return false
	}

	for _, av := range a {
		if !contains(av, b) {
			return false
		}
	}
	return true
}
