package test

import (
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// newMockEngineResponse creates a mock engine response with the given rule status.
func newMockEngineResponse(ruleName string, status engineapi.RuleStatus) engineapi.EngineResponse {
	mockResource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
		},
	}

	mockPolicy := &kyvernov1.ClusterPolicy{}
	mockPolicy.SetName("test-policy")

	ruleResp := engineapi.NewRuleResponse(ruleName, engineapi.Validation, "message", status, nil)
	policyResp := engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{*ruleResp},
	}

	engineResp := engineapi.NewEngineResponse(mockResource, engineapi.NewKyvernoPolicy(mockPolicy), nil)
	engineResp.PolicyResponse = policyResp
	return engineResp
}

// TestPrintTestResult_WantFailGotPass verifies that when a test expects "fail"
// but the policy actually passes, the CLI correctly reports this as a test failure.
func TestPrintTestResult_WantFailGotPass(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	engineResp := newMockEngineResponse("test-rule", engineapi.RuleStatusPass)

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure, "expected test to report failure when want=fail got=pass")
	assert.Equal(t, "Want fail, got pass", resultsTable.RawRows[0].Reason)
}

// TestPrintTestResult_WantFailGotSkip verifies that when a test expects "fail"
// but the policy actually skips, the CLI correctly reports this as a test failure.
func TestPrintTestResult_WantFailGotSkip(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	engineResp := newMockEngineResponse("test-rule", engineapi.RuleStatusSkip)

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure, "expected test to report failure when want=fail got=skip")
	assert.Equal(t, "Want fail, got skip", resultsTable.RawRows[0].Reason)
}

// TestPrintTestResult_WantFailGotWarn verifies that when a test expects "fail"
// but the policy actually warns, the CLI correctly reports this as a test failure.
func TestPrintTestResult_WantFailGotWarn(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	engineResp := newMockEngineResponse("test-rule", engineapi.RuleStatusWarn)

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure, "expected test to report failure when want=fail got=warn")
	assert.Equal(t, "Want fail, got warn", resultsTable.RawRows[0].Reason)
}

// TestPrintTestResult_WantFailGotError verifies that when a test expects "fail"
// but the policy actually errors, the CLI correctly reports this as a test failure.
func TestPrintTestResult_WantFailGotError(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	engineResp := newMockEngineResponse("test-rule", engineapi.RuleStatusError)

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure, "expected test to report failure when want=fail got=error")
	assert.Equal(t, "Want fail, got error", resultsTable.RawRows[0].Reason)
}

// TestPrintTestResult_WantFailGotFail verifies that when a test expects "fail"
// and the policy actually fails, the CLI correctly reports this as a test pass.
func TestPrintTestResult_WantFailGotFail(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	engineResp := newMockEngineResponse("test-rule", engineapi.RuleStatusFail)

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.False(t, resultsTable.RawRows[0].IsFailure, "expected test to pass when want=fail got=fail")
}
