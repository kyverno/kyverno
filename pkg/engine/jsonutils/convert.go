package jsonutils

import jsonutils "github.com/kyverno/kyverno/pkg/utils/json"

// DocumentToUntyped converts a typed object to JSON data
// i.e. string, []interface{}, map[string]interface{}
func DocumentToUntyped(doc interface{}) (interface{}, error) {
	jsonDoc, err := jsonutils.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var untyped interface{}
	err = jsonutils.Unmarshal(jsonDoc, &untyped)
	if err != nil {
		return nil, err
	}

	return untyped, nil
}
