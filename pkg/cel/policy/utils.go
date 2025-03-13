package policy

import "encoding/json"

func getValue(data any) (map[string]interface{}, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	apiData := map[string]interface{}{}
	err = json.Unmarshal(raw, &apiData)
	if err != nil {
		return nil, err
	}
	return apiData, nil
}
