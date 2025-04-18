package policy

import "encoding/json"

func getValue(data any) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	apiData := map[string]any{}
	err = json.Unmarshal(raw, &apiData)
	if err != nil {
		return nil, err
	}
	return apiData, nil
}
