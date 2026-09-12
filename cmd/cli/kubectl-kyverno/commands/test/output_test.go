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
)

func TestPrintTestResult_WantFailGotExcluded(t *testing.T) {
	color.Init(true)
	test := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreports.StatusFail,
		},
		TestResultData: v1alpha1.TestResultData{
			Resources: []string{"test-resource"},
		},
	}
	
	mockPolicy := &kyvernov1.ClusterPolicy{}
	mockPolicy.SetName("test-policy")
	
	// Create a response with empty rules, simulating an excluded resource
	response := engineapi.EngineResponse{
		PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{},
		},
	}
	response = response.WithPolicy(engineapi.NewKyvernoPolicy(mockPolicy))
	
	responses := &TestResponse{
		Target: map[string][]engineapi.EngineResponse{},
		Trigger: map[string][]engineapi.EngineResponse{
			"v1,Pod,default,test-resource": {response}, 
		},
	}
	
	var rc resultCounts
	resultsTable := &table.Table{}
	fs := memfs.New()
	
	err := printTestResult([]v1alpha1.TestResult{test}, responses, &rc, resultsTable, fs, "", true)
	assert.NoError(t, err)
	
	assert.Equal(t, 1, len(resultsTable.RawRows))
	assert.True(t, resultsTable.RawRows[0].IsFailure, "expected test to report failure when want=fail got=excluded")
	assert.Equal(t, "Excluded", resultsTable.RawRows[0].Message)
}
