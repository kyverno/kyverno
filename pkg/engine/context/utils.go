package context

import (
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
)

type jsonAdder interface {
	addJSON(dataMap map[string]interface{}, overwriteMaps bool) error
}

// JSONAdder represents an optional capability for context implementations that support merging raw JSON data.
type JSONAdder interface {
	AddJSON(dataMap map[string]interface{}, overwriteMaps bool) error
}

// AddJSONObject merges json data into the context if supported, or returns an error if unsupported.
func AddJSONObject(ctx Interface, data map[string]interface{}) error {
	if ja, ok := ctx.(jsonAdder); ok {
		return ja.addJSON(data, false)
	}
	if ja, ok := ctx.(JSONAdder); ok {
		return ja.AddJSON(data, false)
	}
	return errors.New("context does not support JSON insertion")
}

// AddResource unmarshals and adds a resource to the context.
func AddResource(ctx Interface, dataRaw []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	return ctx.AddResource(data)
}

// AddOldResource unmarshals and adds an old resource to the context.
func AddOldResource(ctx Interface, dataRaw []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	return ctx.AddOldResource(data)
}

func addToContext(ctx *context, data interface{}, overwriteMaps bool, tags ...string) error {
	if v, err := convertStructs(data); err != nil {
		return err
	} else {
		dataRaw := push(v, tags...)
		return ctx.addJSON(dataRaw, overwriteMaps)
	}
}

func clearLeafValue(data map[string]interface{}, tags ...string) bool {
	if len(tags) == 0 {
		return false
	}

	for i := 0; i < len(tags); i++ {
		k := tags[i]
		if i == len(tags)-1 {
			delete(data, k)
			return true
		}

		if nextMap, ok := data[k].(map[string]interface{}); ok {
			data = nextMap
		} else {
			return false
		}
	}

	return false
}

// convertStructs converts structs, and pointers-to-structs, to map[string]interface{}
func convertStructs(value interface{}) (interface{}, error) {
	if value != nil {
		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Struct {
			return toUnstructured(value)
		}

		if v.Kind() == reflect.Ptr {
			ptrVal := v.Elem()
			if ptrVal.Kind() == reflect.Struct {
				return toUnstructured(value)
			}
		}
	}

	return value, nil
}

func push(data interface{}, tags ...string) map[string]interface{} {
	for i := len(tags) - 1; i >= 0; i-- {
		data = map[string]interface{}{
			tags[i]: data,
		}
	}

	return data.(map[string]interface{})
}

// mergeMaps merges srcMap entries into destMap
func mergeMaps(srcMap, destMap map[string]interface{}, overwriteMaps bool) {
	for k, v := range srcMap {
		if nextSrcMap, ok := v.(map[string]interface{}); ok && !overwriteMaps {
			if nextDestMap, ok := destMap[k].(map[string]interface{}); ok {
				mergeMaps(nextSrcMap, nextDestMap, overwriteMaps)
			} else {
				destMap[k] = nextSrcMap
			}
		} else {
			destMap[k] = v
		}
	}
}

// toUnstructured converts a struct with JSON tags to a map[string]interface{}
func toUnstructured(typedStruct interface{}) (map[string]interface{}, error) {
	converter := runtime.DefaultUnstructuredConverter
	u, err := converter.ToUnstructured(typedStruct)
	return u, err
}
