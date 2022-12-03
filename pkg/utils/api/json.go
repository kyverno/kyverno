package api

import (
	"encoding/json"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// Deserialize "apiextensions.JSON" to a typed array
func DeserializeJSONArray[T any](j apiextensions.JSON) ([]T, error) {
	if j == nil {
		return nil, nil
	}

	data, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}

	var res []T
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	return res, nil
}
