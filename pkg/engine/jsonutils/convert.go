package jsonutils

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DocumentToUntyped converts a typed object to JSON data
// i.e. string, []interface{}, map[string]interface{}
func DocumentToUntyped(doc interface{}) (interface{}, error) {
	jsonDoc, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var untyped interface{}
	err = json.Unmarshal(jsonDoc, &untyped)
	if err != nil {
		return nil, err
	}

	return untyped, nil
}
