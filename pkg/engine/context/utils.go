package context

import (
	"encoding/json"
)

// AddJSONObject merges json data
func AddJSONObject(ctx Interface, data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ctx.addJSON(jsonBytes)
}

func AddResource(ctx Interface, dataRaw []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	return ctx.AddResource(data)
}

func AddOldResource(ctx Interface, dataRaw []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		logger.Error(err, "failed to unmarshal the resource")
		return err
	}
	return ctx.AddOldResource(data)
}

func addToContext(ctx *context, data interface{}, tags ...string) error {
	dataRaw, err := json.Marshal(push(data, tags...))
	if err != nil {
		logger.Error(err, "failed to marshal the resource")
		return err
	}
	return ctx.addJSON(dataRaw)
}

func push(data interface{}, tags ...string) interface{} {
	for i := len(tags) - 1; i >= 0; i-- {
		data = map[string]interface{}{
			tags[i]: data,
		}
	}
	return data
}
