package v1

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func Test_ConditionsWrapper_UnmarshalJSON(t *testing.T) {

	type testCase struct {
		name          string
		conditions    []byte
		isValid       bool
		expectedError string
	}

	testCases := []testCase{
		{
			name: "valid conditions",
			conditions: []byte(`
				[
					{
						"key": "test",
						"operator": "Equals",
						"value": "test"
					}
				]
			`),
			isValid:       true,
			expectedError: "",
		},
		{
			name: "invalid conditions with apiCall",
			conditions: []byte(`
				[
					{
						"key": "test",
						"operator": "Equals",	
						"value": "test",
						"additionalProperties": []
					}
				]
			`),
			isValid:       false,
			expectedError: "json: unknown field \"additionalProperties\"",
		},
		{
			name: "valid any conditions",
			conditions: []byte(`
				{
					"any": [
						{
							"key": "test",
							"operator": "Equals",
							"value": "test"
						}
					]
				}
		`),
			isValid:       true,
			expectedError: "",
		},
		{
			name: "invalid all conditions",
			conditions: []byte(`
				{
					"all": [
						{
							"key": "test",
							"operator": "Equals",
							"value": "test",
							"additionalProperties": []
						}
					]
				}
			`),
			isValid:       false,
			expectedError: "json: unknown field \"additionalProperties\"",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var conditions ConditionsWrapper
			err := json.Unmarshal(testCase.conditions, &conditions)
			if testCase.isValid {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, testCase.expectedError)
			}
		})
	}
}
