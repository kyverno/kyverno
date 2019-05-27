package engine

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func TestValidateValueWithPattern_Bool(t *testing.T) {
	assert.Assert(t, ValidateValueWithPattern(true, true))
	assert.Assert(t, !ValidateValueWithPattern(true, false))
	assert.Assert(t, !ValidateValueWithPattern(false, true))
	assert.Assert(t, ValidateValueWithPattern(false, false))
}

func TestValidateValueWithPattern_BoolInJson(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": true
	}
	`)

	rawValue := []byte(`
	{
		"key": true
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_NullPatternStringValue(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": null
	}
	`)

	rawValue := []byte(`
	{
		"key": "value"
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, !ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_NullPatternDefaultString(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": null
	}
	`)

	rawValue := []byte(`
	{
		"key": ""
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_NullPatternDefaultFloat(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": null
	}
	`)

	rawValue := []byte(`
	{
		"key": 0.0
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_NullPatternDefaultInt(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": null
	}
	`)

	rawValue := []byte(`
	{
		"key": 0
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_NullPatternDefaultBool(t *testing.T) {
	rawPattern := []byte(`
	{
		"key": null
	}
	`)

	rawValue := []byte(`
	{
		"key": false
	}
	`)

	var pattern, value map[string]interface{}
	err := json.Unmarshal(rawPattern, &pattern)
	assert.Assert(t, err)
	err = json.Unmarshal(rawValue, &value)
	assert.Assert(t, err)

	assert.Assert(t, ValidateValueWithPattern(value["key"], pattern["key"]))
}

func TestValidateValueWithPattern_StringsLogicalOr(t *testing.T) {
	pattern := "192.168.88.1 | 10.100.11.*"
	value := "10.100.11.54"
	assert.Assert(t, ValidateValueWithPattern(value, pattern))
}

func TestValidateValueWithNilPattern_NullPatternStringValue(t *testing.T) {
	assert.Assert(t, !validateValueWithNilPattern("value"))
}

func TestValidateValueWithNilPattern_NullPatternDefaultString(t *testing.T) {
	assert.Assert(t, validateValueWithNilPattern(""))
}

func TestValidateValueWithNilPattern_NullPatternDefaultFloat(t *testing.T) {
	assert.Assert(t, validateValueWithNilPattern(0.0))
}

func TestValidateValueWithNilPattern_NullPatternFloat(t *testing.T) {
	assert.Assert(t, !validateValueWithNilPattern(0.1))
}

func TestValidateValueWithNilPattern_NullPatternDefaultInt(t *testing.T) {
	assert.Assert(t, validateValueWithNilPattern(0))
}

func TestValidateValueWithNilPattern_NullPatternInt(t *testing.T) {
	assert.Assert(t, !validateValueWithNilPattern(1))
}

func TestValidateValueWithNilPattern_NullPatternDefaultBool(t *testing.T) {
	assert.Assert(t, validateValueWithNilPattern(false))
}

func TestValidateValueWithNilPattern_NullPatternTrueBool(t *testing.T) {
	assert.Assert(t, !validateValueWithNilPattern(true))
}

func TestValidateValueWithFloatPattern_FloatValue(t *testing.T) {
	assert.Assert(t, validateValueWithFloatPattern(7.9914, 7.9914))
}

func TestValidateValueWithFloatPattern_FloatValueNotPass(t *testing.T) {
	assert.Assert(t, !validateValueWithFloatPattern(7.9914, 7.99141))
}

func TestValidateValueWithFloatPattern_FloatPatternWithoutFractionIntValue(t *testing.T) {
	assert.Assert(t, validateValueWithFloatPattern(7, 7.000000))
}

func TestValidateValueWithFloatPattern_FloatPatternWithoutFraction(t *testing.T) {
	assert.Assert(t, validateValueWithFloatPattern(7.000000, 7.000000))
}

func TestValidateValueWithIntPattern_FloatValueWithoutFraction(t *testing.T) {
	assert.Assert(t, validateValueWithFloatPattern(7.000000, 7))
}

func TestValidateValueWithIntPattern_FloatValueWitFraction(t *testing.T) {
	assert.Assert(t, !validateValueWithFloatPattern(7.000001, 7))
}

func TestValidateValueWithIntPattern_NotPass(t *testing.T) {
	assert.Assert(t, !validateValueWithFloatPattern(8, 7))
}
