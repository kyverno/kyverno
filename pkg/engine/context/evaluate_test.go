package context

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
)

func TestHasChanged(t *testing.T) {
	ctx := createTestContext(`{"a": {"b": 1, "c": 2}, "d": 3}`, `{"a": {"b": 2, "c": 2}, "d": 4}`)

	val, err := ctx.HasChanged("a.b")
	assert.NoError(t, err)
	assert.True(t, val)

	val, err = ctx.HasChanged("a.c")
	assert.NoError(t, err)
	assert.False(t, val)

	val, err = ctx.HasChanged("d")
	assert.NoError(t, err)
	assert.True(t, val)

	_, err = ctx.HasChanged("a.x.y")
	assert.Error(t, err)
}

func TestRequestNotInitialize(t *testing.T) {
	request := admissionv1.AdmissionRequest{}
	ctx := NewContext(jp)
	ctx.AddRequest(request)

	_, err := ctx.HasChanged("x.y.z")
	assert.Error(t, err)
}

func TestMissingOldObject(t *testing.T) {
	request := admissionv1.AdmissionRequest{}
	ctx := NewContext(jp)
	ctx.AddRequest(request)
	request.Object.Raw = []byte(`{"a": {"b": 1, "c": 2}, "d": 3}`)

	_, err := ctx.HasChanged("a.b")
	assert.Error(t, err)
}

func TestMissingObject(t *testing.T) {
	request := admissionv1.AdmissionRequest{}
	ctx := NewContext(jp)
	ctx.AddRequest(request)
	request.OldObject.Raw = []byte(`{"a": {"b": 1, "c": 2}, "d": 3}`)

	_, err := ctx.HasChanged("a.b")
	assert.Error(t, err)
}

func createTestContext(obj, oldObj string) Interface {
	request := admissionv1.AdmissionRequest{}
	request.Operation = "UPDATE"
	request.Object.Raw = []byte(obj)
	request.OldObject.Raw = []byte(oldObj)

	ctx := NewContext(jp)
	ctx.AddRequest(request)
	return ctx
}

func TestQueryOperation(t *testing.T) {
	ctx := createTestContext(`{"a": {"b": 1, "c": 2}, "d": 3}`, `{"a": {"b": 2, "c": 2}, "d": 4}`)
	assert.Equal(t, ctx.QueryOperation(), "UPDATE")
	request := admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
	}

	err := ctx.AddRequest(request)
	assert.Nil(t, err)
	assert.Equal(t, ctx.QueryOperation(), "DELETE")

	err = ctx.AddOperation(string(kyvernov1.Connect))
	assert.Nil(t, err)
	assert.Equal(t, ctx.QueryOperation(), "CONNECT")

	err = ctx.AddRequest(admissionv1.AdmissionRequest{})
	assert.Nil(t, err)
	assert.Equal(t, ctx.QueryOperation(), "")
}

// TestQueryLogicalFallbacks proves the new JMESPath || evaluation works correctly
func TestQueryLogicalFallbacks(t *testing.T) {
	// Mock JSON where 'name' is falsey ("") and 'items' contains an object
	mockJSON := `{"name": "", "items": [{"name": "other"}]}`
	ctx := createTestContext(mockJSON, `{}`)

	testCases := []struct {
		name     string
		query    string
		expected interface{}
	}{
		{
			name:     "fallback missing left, constant right",
			query:    "request.object.doesnotexist || 'x'",
			expected: "x",
		},
		{
			name:     "fallback falsey left, missing right yields null",
			query:    "request.object.name || request.object.doesnotexist",
			expected: nil,
		},
		{
			name:     "fallback truthy middle",
			query:    "request.object.doesnotexist || 'truthy' || 'ignored'",
			expected: "truthy",
		},
		{
			name: "fallback respects nesting and strings",
			// Explicitly using 'missing_items' to guarantee a NotFoundError is thrown for the left side
			query:    "request.object.missing_items[?name == 'a||b'] || 'fallback'",
			expected: "fallback",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ctx.Query(tc.query)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
