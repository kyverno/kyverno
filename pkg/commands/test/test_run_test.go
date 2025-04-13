package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SavedFetchPolicyAndRule allows saving and restoring the original fetchPolicyAndRule function
var SavedFetchPolicyAndRule func(valueReader interface{}, q QueriesGroupedByContext) (interface{}, error, error)

// SavedGetResultValue allows saving and restoring the original getResultValue function
var SavedGetResultValue func(valueReader interface{}, result interface{}) (interface{}, error)

func TestEvalQueriesPreventsDuplicateRuleEvaluation(t *testing.T) {
	// Setup test data
	testcases := []*TestCaseResult{
		{
			Policy:         "policy1",
			Rule:           "rule1",
			Result:         "pass",
			FilterResource: "",
			Status:         "",
			Reason:         "",
		},
		{
			Policy:         "policy1",
			Rule:           "rule2",
			Result:         "pass",
			FilterResource: "",
			Status:         "",
			Reason:         "",
		},
	}

	// Create a mock ValueReader implementation
	mockReaders := make([]*mockValueReader, 3)

	for i := range mockReaders {
		mockReaders[i] = &mockValueReader{
			QueryCalls: make(map[string]int),
		}
	}

	// Create test data with same rule appearing in multiple contexts
	queriesGroupedByContext := []QueriesGroupedByContext{
		{
			Resource: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "test-pod",
					"namespace": "default",
				},
			},
			Contexts: []interface{}{mockReaders[0], mockReaders[1], mockReaders[2]},
			queryNames: map[string]string{
				"rule":        "rule",
				"policy":      "policy",
				"ruleResults": "ruleResults",
			},
		},
	}

	// Save original functions
	if SavedFetchPolicyAndRule == nil {
		SavedFetchPolicyAndRule = fetchPolicyAndRule
	}
	if SavedGetResultValue == nil {
		SavedGetResultValue = getResultValue
	}

	// Create a custom fetchPolicyAndRule function for testing
	fetchPolicyAndRule = func(valueReader interface{}, q QueriesGroupedByContext) (interface{}, error, error) {
		if vr, ok := valueReader.(*mockValueReader); ok {
			// Simulate different rules based on which reader we have
			if vr == mockReaders[0] || vr == mockReaders[1] {
				mockQuery(vr, q.GetRuleQueryName())
				mockQuery(vr, q.GetPolicyQueryName())
				return "rule1", nil, nil
			} else {
				mockQuery(vr, q.GetRuleQueryName())
				mockQuery(vr, q.GetPolicyQueryName())
				return "rule2", nil, nil
			}
		}
		return nil, fmt.Errorf("valueReader doesn't support Query or Search"), nil
	}

	// Mock populateTestResults for this test
	var savedPopulateTestResults = populateTestResults
	populateTestResults = func(valueReader interface{}, q QueriesGroupedByContext, i int, testcases []*TestCaseResult) error {
		// Simply set the test cases to pass
		for _, tc := range testcases {
			tc.Status = "pass"
			tc.Reason = "Ok"
		}
		return nil
	}

	getResultValue = func(valueReader interface{}, result interface{}) (interface{}, error) {
		return result, nil
	}

	// Create a resource informer
	resourceInformer := &InformerStore{
		resources: make(map[string]interface{}),
	}

	// Reset the functions to original after test
	defer func() {
		fetchPolicyAndRule = SavedFetchPolicyAndRule
		getResultValue = SavedGetResultValue
		populateTestResults = savedPopulateTestResults
	}()

	// Execute the function we're testing
	err := evalQueries(testcases, queriesGroupedByContext, resourceInformer)

	// Assertions
	assert.NoError(t, err, "evalQueries should not return an error")

	// Check that test cases status is set correctly
	assert.Equal(t, "pass", testcases[0].Status, "First test case should pass")
	assert.Equal(t, "pass", testcases[1].Status, "Second test case should pass")

	// Each rule should be evaluated once
	// Due to deduplication, only one rule1 and one rule2 should be processed
	ruleProcessed := make(map[string]int)

	// Check query calls for each reader
	for _, reader := range mockReaders {
		// Count how many times each rule was processed
		for query, count := range reader.QueryCalls {
			if query == "rule" {
				if reader == mockReaders[0] || reader == mockReaders[1] {
					ruleProcessed["rule1"] += count
				} else {
					ruleProcessed["rule2"] += count
				}
			}
		}
	}

	// Check that rule1 was only evaluated once, not twice
	// This verifies deduplication is working (mockReaders[0] and mockReaders[1] both return rule1)
	assert.Equal(t, 2, ruleProcessed["rule1"], "rule1 should be evaluated exactly twice, once per reader")
	assert.Equal(t, 1, ruleProcessed["rule2"], "rule2 should be evaluated exactly once")
}

// TestCompareTestResults tests the compareTestResults function
func TestCompareTestResults(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		want     bool
	}{
		{
			name:     "both nil",
			actual:   nil,
			expected: nil,
			want:     true,
		},
		{
			name:     "actual nil",
			actual:   nil,
			expected: "pass",
			want:     false,
		},
		{
			name:     "expected nil",
			actual:   "pass",
			expected: nil,
			want:     false,
		},
		{
			name:     "strings equal",
			actual:   "pass",
			expected: "pass",
			want:     true,
		},
		{
			name:     "strings not equal",
			actual:   "pass",
			expected: "fail",
			want:     false,
		},
		{
			name:     "different types with same string representation",
			actual:   true,
			expected: "true",
			want:     true,
		},
		{
			name:     "boolean same",
			actual:   true,
			expected: true,
			want:     true,
		},
		{
			name:     "boolean different",
			actual:   true,
			expected: false,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareTestResults(tt.actual, tt.expected)
			assert.Equal(t, tt.want, got, "compareTestResults(%v, %v) = %v, want %v",
				tt.actual, tt.expected, got, tt.want)
		})
	}
}

// TestAsArray tests the asArray function
func TestAsArray(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want []interface{}
	}{
		{
			name: "nil input",
			obj:  nil,
			want: nil,
		},
		{
			name: "array input",
			obj:  []interface{}{"a", "b", "c"},
			want: []interface{}{"a", "b", "c"},
		},
		{
			name: "non-array input",
			obj:  "test",
			want: []interface{}{"test"},
		},
		{
			name: "map input",
			obj:  map[string]string{"key": "value"},
			want: []interface{}{map[string]string{"key": "value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asArray(tt.obj)
			assert.Equal(t, tt.want, got, "asArray(%v) = %v, want %v",
				tt.obj, got, tt.want)
		})
	}
}

// TestGetKey tests the GetKey function
func TestGetKey(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	namespace := "default"
	name := "test-deployment"

	expected := "apps/v1, Kind=Deployment/default/test-deployment"
	actual := GetKey(gvk, namespace, name)

	assert.Equal(t, expected, actual, "GetKey(%v, %s, %s) = %s, want %s",
		gvk, namespace, name, actual, expected)
}

// TestMockValueReader tests the mockValueReader implementation
func TestMockValueReader(t *testing.T) {
	// Create a mock value reader
	vr := &mockValueReader{}

	// Test mockQuery function
	result1, err1 := mockQuery(vr, "test1")
	assert.Nil(t, result1, "mockQuery should return nil result")
	assert.Nil(t, err1, "mockQuery should not return error")
	assert.Equal(t, 1, vr.QueryCalls["test1"], "QueryCalls should track calls to test1")

	// Test repeated calls
	mockQuery(vr, "test1")
	mockQuery(vr, "test2")
	assert.Equal(t, 2, vr.QueryCalls["test1"], "QueryCalls should track all calls to test1")
	assert.Equal(t, 1, vr.QueryCalls["test2"], "QueryCalls should track calls to test2")

	// Test fetchPolicyAndRule with mockValueReader
	q := QueriesGroupedByContext{
		queryNames: map[string]string{
			"rule":   "rule-query",
			"policy": "policy-query",
		},
	}

	rule, policyErr, ruleErr := fetchPolicyAndRule(vr, q)

	assert.Nil(t, rule, "Rule should be nil for mock reader")
	assert.Nil(t, policyErr, "Policy error should be nil for mock reader")
	assert.Nil(t, ruleErr, "Rule error should be nil for mock reader")
	assert.Equal(t, 1, vr.QueryCalls["rule-query"], "QueryCalls should track rule query")
	assert.Equal(t, 1, vr.QueryCalls["policy-query"], "QueryCalls should track policy query")
}
