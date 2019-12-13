package mergo

import (
	"encoding/json"
	"testing"
)

var (
	data = `{"FirstSlice":[], "SecondSlice": null}`
)

type settings struct {
	FirstSlice  []string `json:"FirstSlice"`
	SecondSlice []string `json:"SecondSlice"`
}

func TestIssue125MergeWithOverwrite(t *testing.T) {

	defaultSettings := settings{
		FirstSlice:  []string{},
		SecondSlice: []string{},
	}

	var something settings
	if err := json.Unmarshal([]byte(data), &something); err != nil {
		t.Errorf("Error while Unmarshalling maprequest: %s", err)
	}
	if err := Merge(&something, defaultSettings, WithOverrideEmptySlice); err != nil {
		t.Errorf("Error while merging: %s", err)
	}
	if something.FirstSlice == nil {
		t.Error("Invalid merging first slice")
	}
	if something.SecondSlice == nil {
		t.Error("Invalid merging second slice")
	}
}
