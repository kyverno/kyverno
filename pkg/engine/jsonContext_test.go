package engine

import (
	"bytes"
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func Test_parseMultilineBlockBody(t *testing.T) {
	tcs := []struct {
		multilineBlockRaw         []byte
		expectedMultilineBlockRaw []byte
		expectedErr               bool
	}{
		{
			multilineBlockRaw: []byte(`{
				"key1": "value",
				"key2": "value2",
				"key3": "word1\nword2\nword3",
				"key4": "word4\n"
			}`),
			expectedMultilineBlockRaw: []byte(`{"key1":"value","key2":"value2","key3":["word1","word2","word3"],"key4":"word4"}`),
			expectedErr:               false,
		},
		{
			multilineBlockRaw: []byte(`{
				"key1": "value",
				"key2": "value2",
				"key3": "word1\nword2\nword3",
				"key4": "word4"
			}`),
			expectedMultilineBlockRaw: []byte(`{"key1":"value","key2":"value2","key3":["word1","word2","word3"],"key4":"word4"}`),
			expectedErr:               false,
		},
		{
			multilineBlockRaw: []byte(`{
				"key1": "value1",
				"key2": "value2\n",
				"key3": "word1",
				"key4": "word2"
			}`),
			expectedMultilineBlockRaw: []byte(`{"key1":"value1","key2":["value2",""]}`),
			expectedErr:               true,
		},
		{
			multilineBlockRaw: []byte(`{
				"key1": "value1",
				"key2": "[\"cluster-admin\", \"cluster-operator\", \"tenant-admin\"]"
			}`),
			expectedMultilineBlockRaw: []byte(`{"key1":"value1","key2":"[\"cluster-admin\", \"cluster-operator\", \"tenant-admin\"]"}`),
			expectedErr:               false,
		},
		{
			multilineBlockRaw: []byte(`{
				"key1": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHMmDjK65krAyDaGaeyWNzgvIu155\nJI50B2vezCw8+3CVeE0lJTL5dbL3OP98Za0oAEBJcOxky8Riy/XcmfKZbw==\n-----END PUBLIC KEY-----"
			}`),
			expectedMultilineBlockRaw: []byte(`{"key1":"-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHMmDjK65krAyDaGaeyWNzgvIu155\nJI50B2vezCw8+3CVeE0lJTL5dbL3OP98Za0oAEBJcOxky8Riy/XcmfKZbw==\n-----END PUBLIC KEY-----"}`),
			expectedErr:               false,
		},
	}

	for _, tc := range tcs {
		var multilineBlock map[string]interface{}
		err := json.Unmarshal(tc.multilineBlockRaw, &multilineBlock)
		assert.NilError(t, err)

		parsedMultilineBlock := parseMultilineBlockBody(multilineBlock)
		parsedMultilineBlockRaw, err := json.Marshal(parsedMultilineBlock)
		assert.NilError(t, err)

		if tc.expectedErr {
			assert.Assert(t, bytes.Compare(parsedMultilineBlockRaw, tc.expectedMultilineBlockRaw) != 0)
		} else {
			assert.Assert(t, bytes.Compare(parsedMultilineBlockRaw, tc.expectedMultilineBlockRaw) == 0)
		}
	}
}
