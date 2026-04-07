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

func TestPrintTestResult_WantFailGotPass(t *testing.T) {
	color.Init(true)
	// Setup dummy test values
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail, // We EXPECT it to fail
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"v1/Pod/default/test-resource"},
		},
	}

	// Mocking response where the policy actually passed
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

	ruleResp := engineapi.NewRuleResponse("test-rule", engineapi.Validation, "message", engineapi.RuleStatusPass, nil)
	policyResp := engineapi.PolicyResponse{
		Rules: []engineapi.RuleResponse{*ruleResp},
	}

	engineResp := engineapi.NewEngineResponse(mockResource, engineapi.NewKyvernoPolicy(mockPolicy), nil)
	engineResp.PolicyResponse = policyResp

	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {engineResp},
		},
	}

	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()

	// Call the function
	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)

	// Since expected is FAIL but actual was PASS, the table should contain a failure row with our specific reason
	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure)
	assert.Equal(t, "Want fail, got pass", resultsTable.RawRows[0].Reason)
}
