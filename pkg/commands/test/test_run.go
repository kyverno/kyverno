package test

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// StringQuery represents a string-based JMESPath query
type StringQuery string

// Query executes the JMESPath query
func (s StringQuery) Query(reader jmespath.Interface) (interface{}, error) {
	return reader.Search(string(s), nil)
}

// String returns the string representation of the query
func (s StringQuery) String() string {
	return string(s)
}

// TestCaseResult represents the result of a test case
type TestCaseResult struct {
	Policy         interface{}
	Rule           interface{}
	Result         interface{}
	FilterResource interface{}
	Status         string
	Reason         string
}

// TestStatus represents the status of a test
type TestStatus string

const (
	// TestPassed indicates the test passed
	TestPassed TestStatus = "pass"
	// TestFailed indicates the test failed
	TestFailed TestStatus = "fail"
)

// QueriesGroupedByContext represents queries grouped by context
type QueriesGroupedByContext struct {
	Resource   interface{}
	Contexts   []interface{}
	queryNames map[string]string
}

// GetRuleQueryName returns the rule query name
func (q QueriesGroupedByContext) GetRuleQueryName() string {
	return q.queryNames["rule"]
}

// GetPolicyQueryName returns the policy query name
func (q QueriesGroupedByContext) GetPolicyQueryName() string {
	return q.queryNames["policy"]
}

// GetRuleResultsQueryName returns the rule results query name
func (q QueriesGroupedByContext) GetRuleResultsQueryName() string {
	return q.queryNames["ruleResults"]
}

// GetResource returns the resource
func (q QueriesGroupedByContext) GetResource() interface{} {
	return q.Resource
}

// GetAPIVersion returns the API version of the resource
func (q QueriesGroupedByContext) GetAPIVersion() string {
	resourceMap, ok := q.Resource.(map[string]interface{})
	if !ok {
		return ""
	}
	apiVersion, ok := resourceMap["apiVersion"].(string)
	if !ok {
		return ""
	}
	return apiVersion
}

// GetKind returns the kind of the resource
func (q QueriesGroupedByContext) GetKind() string {
	resourceMap, ok := q.Resource.(map[string]interface{})
	if !ok {
		return ""
	}
	kind, ok := resourceMap["kind"].(string)
	if !ok {
		return ""
	}
	return kind
}

// GetNamespace returns the namespace of the resource
func (q QueriesGroupedByContext) GetNamespace() string {
	resourceMap, ok := q.Resource.(map[string]interface{})
	if !ok {
		return ""
	}
	metadata, ok := resourceMap["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	namespace, ok := metadata["namespace"].(string)
	if !ok {
		return ""
	}
	return namespace
}

// GetName returns the name of the resource
func (q QueriesGroupedByContext) GetName() string {
	resourceMap, ok := q.Resource.(map[string]interface{})
	if !ok {
		return ""
	}
	metadata, ok := resourceMap["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, ok := metadata["name"].(string)
	if !ok {
		return ""
	}
	return name
}

// InformerStore stores resources in the informer
type InformerStore struct {
	resources map[string]interface{}
}

// AddGVK adds a GroupVersionKind to the informer
func (s *InformerStore) AddGVK(gvk schema.GroupVersionKind) {
	if s.resources == nil {
		s.resources = make(map[string]interface{})
	}
}

// Update updates a resource in the informer
func (s *InformerStore) Update(key string, obj interface{}) {
	if s.resources == nil {
		s.resources = make(map[string]interface{})
	}
	s.resources[key] = obj
}

// GetKey returns a key for a GVK and resource
func GetKey(gvk schema.GroupVersionKind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
}

// mockValueReader is used for testing
type mockValueReader struct {
	QueryCalls map[string]int
}

// mockQuery is used for mocking the query function in tests
func mockQuery(vr *mockValueReader, query string) (interface{}, error) {
	if vr.QueryCalls == nil {
		vr.QueryCalls = make(map[string]int)
	}
	vr.QueryCalls[query]++
	return nil, nil
}

// Make these functions variables so they can be mocked in tests
var fetchPolicyAndRule = func(valueReader interface{}, q QueriesGroupedByContext) (interface{}, error, error) {
	// First, check if valueReader supports the mockQuery interface used in tests
	if vr, ok := valueReader.(*mockValueReader); ok {
		rule, _ := mockQuery(vr, q.GetRuleQueryName())
		_, policyErr := mockQuery(vr, q.GetPolicyQueryName())
		return rule, policyErr, nil
	}

	// Use type assertion to access the Query method if it exists
	if reader, ok := valueReader.(interface {
		Query(string) (interface{}, error)
	}); ok {
		rule, ruleErr := reader.Query(q.GetRuleQueryName())
		_, policyErr := reader.Query(q.GetPolicyQueryName())
		return rule, policyErr, ruleErr
	}

	// If valueReader doesn't have a Query method, try Search method
	if reader, ok := valueReader.(interface {
		Search(string, interface{}) (interface{}, error)
	}); ok {
		rule, ruleErr := reader.Search(q.GetRuleQueryName(), nil)
		_, policyErr := reader.Search(q.GetPolicyQueryName(), nil)
		return rule, policyErr, ruleErr
	}

	return nil, fmt.Errorf("valueReader doesn't support Query or Search"), nil
}

var getResultValue = func(valueReader interface{}, result interface{}) (interface{}, error) {
	// Use type assertion to access the Query method if it exists
	if reader, ok := valueReader.(interface {
		Query(interface{}) (interface{}, error)
	}); ok {
		return reader.Query(result)
	}

	// If valueReader doesn't have a Query method, try Search method
	if reader, ok := valueReader.(interface {
		Search(string, interface{}) (interface{}, error)
	}); ok {
		if resultStr, ok := result.(string); ok {
			return reader.Search(resultStr, nil)
		}
	}

	// If both methods failed or result isn't a string, just return the result itself
	return result, nil
}

func evalQueries(testcases []*TestCaseResult, queriesGroupedByContext []QueriesGroupedByContext, resourceInformer *InformerStore) error {
	// add resource to informer
	for _, q := range queriesGroupedByContext {
		r := q.GetResource()
		gvk := schema.FromAPIVersionAndKind(q.GetAPIVersion(), q.GetKind())
		key := GetKey(gvk, q.GetNamespace(), q.GetName())
		resourceInformer.AddGVK(gvk)
		resourceInformer.Update(key, r)
	}

	// Create a map to track processed rule-policy combinations at global level
	processedRules := make(map[string]bool)

	for _, q := range queriesGroupedByContext {
		for i, valueReader := range q.Contexts {
			rule, policyErr, ruleErr := fetchPolicyAndRule(valueReader, q)
			if policyErr != nil || ruleErr != nil {
				continue
			}

			// Create a unique key for this policy-rule combination
			ruleKey := fmt.Sprintf("%v-%v", rule, q.GetPolicyQueryName())

			// Skip if we've already processed this rule
			if processedRules[ruleKey] {
				continue
			}

			// Mark this rule as processed
			processedRules[ruleKey] = true

			err := populateTestResults(valueReader, q, i, testcases)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var populateTestResults = func(valueReader interface{}, q QueriesGroupedByContext, i int, testcases []*TestCaseResult) error {
	var rule, policy, ruleResults interface{}
	var err error

	// Use type assertion to access the Query method if it exists
	if reader, ok := valueReader.(interface {
		Query(string) (interface{}, error)
	}); ok {
		rule, err = reader.Query(q.GetRuleQueryName())
		if err != nil {
			return fmt.Errorf("failed to query rule: %w", err)
		}

		policy, err = reader.Query(q.GetPolicyQueryName())
		if err != nil {
			return fmt.Errorf("failed to query policy: %w", err)
		}

		ruleResults, err = reader.Query(q.GetRuleResultsQueryName())
		if err != nil {
			return fmt.Errorf("failed to query rule results: %w", err)
		}
	} else if reader, ok := valueReader.(interface {
		Search(string, interface{}) (interface{}, error)
	}); ok {
		// If valueReader doesn't have a Query method, try Search method
		rule, err = reader.Search(q.GetRuleQueryName(), nil)
		if err != nil {
			return fmt.Errorf("failed to query rule: %w", err)
		}

		policy, err = reader.Search(q.GetPolicyQueryName(), nil)
		if err != nil {
			return fmt.Errorf("failed to query policy: %w", err)
		}

		ruleResults, err = reader.Search(q.GetRuleResultsQueryName(), nil)
		if err != nil {
			return fmt.Errorf("failed to query rule results: %w", err)
		}
	} else {
		return fmt.Errorf("valueReader doesn't support Query or Search")
	}

	for _, tc := range testcases {
		tcRule := tc.Rule
		tcPolicy := tc.Policy

		if tcPolicy != nil && fmt.Sprintf("%v", policy) != fmt.Sprintf("%v", tcPolicy) {
			continue
		}

		if tcRule != nil && fmt.Sprintf("%v", rule) != fmt.Sprintf("%v", tcRule) {
			continue
		}

		tc.Status = "pass"
		tc.Reason = "Ok"

		val, err := getResultValue(valueReader, tc.Result)
		if err != nil {
			return fmt.Errorf("failed to query result: %w", err)
		}

		if tc.FilterResource != nil && fmt.Sprintf("%v", tc.FilterResource) != "" {
			var found bool
			results := asArray(ruleResults)

			for _, result := range results {
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					continue
				}

				resource, ok := resultMap["resource"].(map[string]interface{})
				if !ok {
					continue
				}

				apiVersion, _ := resource["apiVersion"].(string)
				kind, _ := resource["kind"].(string)
				name, _ := resource["name"].(string)
				resourceStr := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)

				if resourceStr == fmt.Sprintf("%v", tc.FilterResource) {
					found = true
					actual, _ := resultMap["pass"]
					expected := val
					if !compareTestResults(actual, expected) {
						tc.Status = "fail"
						tc.Reason = fmt.Sprintf("Want %v, got %v", expected, actual)
					}
					break
				}
			}

			if !found {
				tc.Status = "fail"
				tc.Reason = fmt.Sprintf("Resource %v not found in rule results", tc.FilterResource)
			}
		} else {
			passes := make([]interface{}, 0)
			results := asArray(ruleResults)

			for _, result := range results {
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					continue
				}

				pass, ok := resultMap["pass"]
				if ok {
					passes = append(passes, pass)
				}
			}

			actual := passes
			expected := val
			if !compareTestResults(actual, expected) {
				tc.Status = "fail"
				tc.Reason = fmt.Sprintf("Want %v, got %v", expected, actual)
			}
		}
	}

	return nil
}

// asArray converts an interface to an array
func asArray(obj interface{}) []interface{} {
	if obj == nil {
		return nil
	}

	if arr, ok := obj.([]interface{}); ok {
		return arr
	}

	return []interface{}{obj}
}

// compareTestResults compares the actual test result with the expected result
func compareTestResults(actual, expected interface{}) bool {
	// Simple equality comparison
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Convert both to strings for comparison
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}
