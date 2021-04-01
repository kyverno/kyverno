package context

import (
	"encoding/json"
	"testing"

	"github.com/jmespath/go-jmespath"
	"gotest.tools/assert"
)

func Test_regexMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "hgf'b1a2r'b12g"

	query, err := jmespath.Compile("regexMatch('12.*', foo)")
	assert.NilError(t, err)

	query.Register(getRegexMatch())

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_regexMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := jmespath.Compile("regexMatch('12.*', abs(foo))")
	assert.NilError(t, err)

	query.Register(getRegexMatch())

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_regexReplaceAll(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "ns_first"
		},
		"spec": {
			"namespace": "ns_first",
			"name": "temp_other",
			"field" : "Hello world, helworldlo"
		}
	}
	`)
	expected := []byte(`Glo world, Gworldlo`)

	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	query, err := jmespath.Compile(`regexReplaceAll('([Hh]e|G)l', spec.field, '${2}G')`)
	assert.NilError(t, err)

	query.Register(getRegexReplaceAll())

	res, err := query.Search(resource)
	assert.NilError(t, err)
	result := res.([]byte)
	assert.Equal(t, string(result), (string(expected)))
}

func Test_regexReplaceAllLiteral(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "ns_first"
		},
		"spec": {
			"namespace": "ns_first",
			"name": "temp_other",
			"field" : "Hello world, helworldlo"
		}
	}
	`)
	expected := []byte(`Glo world, Gworldlo`)

	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)

	query, err := jmespath.Compile(`regexReplaceAllLiteral('[Hh]el?', spec.field, 'G')`)
	assert.NilError(t, err)

	query.Register(getRegexReplaceAllLiteral())

	res, err := query.Search(resource)
	assert.NilError(t, err)
	result := res.([]byte)
	assert.Equal(t, string(result), (string(expected)))
}
