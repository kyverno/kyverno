package context

import (
	"errors"
	"testing"

	gojmespath "github.com/kyverno/go-jmespath"
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

func TestIsTruthy(t *testing.T) {
	testCases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{name: "nil", value: nil, expected: false},
		{name: "true", value: true, expected: true},
		{name: "false", value: false, expected: false},
		{name: "non-empty string", value: "value", expected: true},
		{name: "empty string", value: "", expected: false},
		{name: "non-empty interface slice", value: []interface{}{1}, expected: true},
		{name: "empty interface slice", value: []interface{}{}, expected: false},
		{name: "non-empty interface map", value: map[string]interface{}{"key": "value"}, expected: true},
		{name: "empty interface map", value: map[string]interface{}{}, expected: false},
		{name: "non-empty custom slice", value: []map[string]any{{"key": "value"}}, expected: true},
		{name: "empty custom slice", value: []map[string]any{}, expected: false},
		{name: "non-empty string slice", value: []string{"value"}, expected: true},
		{name: "empty string slice", value: []string{}, expected: false},
		{name: "non-empty custom map", value: map[string]string{"key": "value"}, expected: true},
		{name: "empty custom map", value: map[string]string{}, expected: false},
		{name: "non-empty array", value: [1]string{"value"}, expected: true},
		{name: "empty array", value: [0]string{}, expected: false},
		{name: "nil pointer", value: (*string)(nil), expected: false},
		{name: "non-nil pointer", value: new(string), expected: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isTruthy(tc.value))
		})
	}
}

func TestSafeSplitLogicalOr(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "top-level fallback chain",
			query:    "missing1 || missing2 || 'fallback'",
			expected: []string{"missing1", "missing2", "'fallback'"},
		},
		{
			name:     "operator inside single-quoted string",
			query:    "value || 'a || b'",
			expected: []string{"value", "'a || b'"},
		},
		{
			name:     "operator inside escaped single-quoted string",
			query:    "value || 'a \\' || b'",
			expected: []string{"value", "'a \\' || b'"},
		},
		{
			name:     "operator inside double-quoted identifier",
			query:    `value || "a||b"`,
			expected: []string{"value", `"a||b"`},
		},
		{
			name:     "operator inside backtick literal",
			query:    "value || `{\"key\": \"a || b\"}`",
			expected: []string{"value", "`{\"key\": \"a || b\"}`"},
		},
		{
			name:     "nested filter fallback remains unsplit",
			query:    "items[?missing || name == 'test']",
			expected: []string{"items[?missing || name == 'test']"},
		},
		{
			name:     "pipe in filter is not top-level",
			query:    "items[?value | length(@) > `0`] || 'fallback'",
			expected: []string{"items[?value | length(@) > `0`]", "'fallback'"},
		},
		{
			name:     "top-level pipe before logical or",
			query:    "request.object.spec | foo || replicas",
			expected: []string{"request.object.spec | foo || replicas"},
		},
		{
			name:     "top-level pipe after logical or",
			query:    "a || b | c",
			expected: []string{"a || b | c"},
		},
		{
			name:     "parenthesized pipe is nested",
			query:    "(a | b) || c",
			expected: []string{"(a | b)", "c"},
		},
		{
			name:     "malformed trailing operator preserves query",
			query:    "a || b || ",
			expected: []string{"a || b || "},
		},
		{
			name:     "malformed consecutive operators preserves query",
			query:    "a || || b",
			expected: []string{"a || || b"},
		},
		{
			name:     "malformed leading operator preserves query",
			query:    "|| a",
			expected: []string{"|| a"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, safeSplitLogicalOr(tc.query))
		})
	}
}

func TestQueryTopLevelLogicalFallbacks(t *testing.T) {
	ctx := createTestContext(`{"name": "", "value": "present", "present_but_falsy": ""}`, `{}`)

	// 1. Missing key fallback (Triggers the NotFoundError fallback loop)
	result, err := ctx.Query("request.object.missing || 'fallback'")
	assert.NoError(t, err)
	assert.Equal(t, "fallback", result)

	// 2. Chain of missing keys resolving to a present leaf key
	result, err = ctx.Query("request.object.missing1 || request.object.missing2 || request.object.value || 'fallback'")
	assert.NoError(t, err)
	assert.Equal(t, "present", result)

	// 3. Fallback to a present but falsy value
	result, err = ctx.Query("request.object.missing || request.object.present_but_falsy")
	assert.NoError(t, err)
	assert.Equal(t, "", result)

	// 3b. A present falsy operand followed by a missing operand resolves to nil without an error
	result, err = ctx.Query("request.object.present_but_falsy || request.object.missing")
	assert.NoError(t, err)
	assert.Nil(t, result)

	// 4. All missing keys MUST return an error (verifying the error swallow fix)
	_, err = ctx.Query("request.object.missing1 || request.object.missing2")
	assert.Error(t, err)
	var notFoundErr gojmespath.NotFoundError
	assert.True(t, errors.As(err, &notFoundErr))

	// 5. Single missing key MUST return an error
	_, err = ctx.Query("request.object.missing")
	assert.Error(t, err)
	assert.True(t, errors.As(err, &notFoundErr))

	// 6. Syntax error in a fallback operand MUST return an error (coverage for compile loop)
	_, err = ctx.Query("request.object.missing || invalid_syntax(")
	assert.Error(t, err)

	// 7. An empty slice of maps returned by a custom function is falsy inside a fallback chain
	result, err = ctx.Query("request.object.missing || items(`{}`, `\"key\"`, `\"value\"`) || 'fallback'")
	assert.NoError(t, err)
	assert.Equal(t, "fallback", result)

	// 8. The same empty custom slice ahead of a literal, with no missing operand
	result, err = ctx.Query("items(`{}`, `\"key\"`, `\"value\"`) || 'fallback'")
	assert.NoError(t, err)
	assert.Equal(t, "fallback", result)

	// 9. Runtime error in a fallback operand MUST propagate immediately and not fall back
	_, err = ctx.Query("request.object.missing || length(`1`) || 'fallback'")
	assert.Error(t, err)
	assert.False(t, errors.As(err, &notFoundErr))
}
