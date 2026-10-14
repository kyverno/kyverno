package test

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestPrintCheckResult(t *testing.T) {
	color.Init(true)
	newResponse := func(resource string, rule engineapi.RuleResponse) engineapi.EngineResponse {
		response := engineapi.NewEngineResponse(
			unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata":   map[string]any{"name": resource, "namespace": "default"},
			}},
			engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team-label"}}),
			nil,
		)
		response.PolicyResponse.Rules = []engineapi.RuleResponse{rule}
		return response
	}
	responses := TestResponse{Trigger: map[string][]engineapi.EngineResponse{
		"v1,Pod,default,bad":  {newResponse("bad", *engineapi.RuleFail("check-team", engineapi.Validation, "The label `team` is required.", nil))},
		"v1,Pod,default,good": {newResponse("good", *engineapi.RuleSkip("check-team", engineapi.Validation, "skipped", nil))},
	}}
	tests := []struct {
		name              string
		check             string
		wantPass          int
		wantSkip          int
		wantFail          int
		wantErr           string
		initialRow        bool
		wantFirstCheckID  int
		checkFailureState bool
		wantRowFailure    bool
	}{{
		name: "assert",
		check: `
match: {resource: {metadata: {name: bad}}, policy: {metadata: {name: require-team-label}}, rule: {name: check-team}}
assert: {status: fail, message: "The label ` + "`team`" + ` is required."}`,
		wantPass: 1,
	}, {
		name:     "assert fails",
		check:    `{match: {resource: {metadata: {name: bad}}}, assert: {message: wrong}}`,
		wantFail: 1,
	}, {
		name:             "continues row IDs after existing test results",
		check:            `{match: {resource: {metadata: {name: bad}}}, assert: {status: fail}}`,
		wantPass:         1,
		initialRow:       true,
		wantFirstCheckID: 2,
	}, {
		name:     "assert expression",
		check:    `{assert: {(object.ruleType == 'Validation'): true}}`,
		wantPass: 1,
		wantSkip: 1,
	}, {
		name:              "error",
		check:             `{match: {resource: {metadata: {name: bad}}}, error: {status: pass}}`,
		wantPass:          1,
		checkFailureState: true,
		wantRowFailure:    false,
	}, {
		name:              "error fails",
		check:             `{match: {resource: {metadata: {name: bad}}}, error: {status: fail}}`,
		wantFail:          1,
		checkFailureState: true,
		wantRowFailure:    true,
	}, {
		name:    "missing assertion",
		check:   `{match: {resource: {metadata: {name: bad}}}}`,
		wantErr: "checks[0]: at least one of assert or error must be specified",
	}, {
		name:  "no match",
		check: `{match: {policy: {metadata: {name: other}}}, assert: {status: fail}}`,
	}, {
		name:    "invalid expression",
		check:   `{assert: {(length(message)): 29}}`,
		wantErr: "JMESPath is no longer supported",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var check v1alpha1.CheckResult
			require.NoError(t, yaml.UnmarshalStrict([]byte(tt.check), &check))
			rc := &resultCounts{}
			var resultsTable table.Table
			if tt.initialRow {
				resultsTable.Add(table.Row{RowCompact: table.RowCompact{ID: 1}})
			}
			err := printCheckResult([]v1alpha1.CheckResult{check}, responses, rc, &resultsTable)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.checkFailureState {
				require.Len(t, resultsTable.RawRows, 1)
				assert.Equal(t, tt.wantRowFailure, resultsTable.RawRows[0].IsFailure)
			}
			if tt.wantFirstCheckID != 0 {
				require.Len(t, resultsTable.RawRows, 2)
				assert.Equal(t, tt.wantFirstCheckID, resultsTable.RawRows[1].ID)
			}
			assert.Equal(t, tt.wantPass, rc.Pass, "pass")
			assert.Equal(t, tt.wantSkip, rc.Skip, "skip")
			assert.Equal(t, tt.wantFail, rc.Fail, "fail")
		})
	}
}

func TestPrintCheckResultIncludesGeneratedAndPatchedFields(t *testing.T) {
	color.Init(true)

	generatedOne := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "generated-one", "namespace": "default"},
	}}
	generatedTwo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "generated-two", "namespace": "default"},
	}}
	patchedTarget := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "patched-target", "namespace": "default"},
	}}
	rule := engineapi.RulePass("check-output", engineapi.Mutation, "target updated", nil).
		WithGeneratedResources([]*unstructured.Unstructured{generatedOne, generatedTwo}).
		WithPatchedTarget(patchedTarget, metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, "scale")
	response := engineapi.NewEngineResponse(
		unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "target", "namespace": "default"},
		}},
		engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "target-policy"}}),
		nil,
	)
	response.PolicyResponse.Rules = []engineapi.RuleResponse{*rule}
	responses := TestResponse{Target: map[string][]engineapi.EngineResponse{
		"v1,Pod,default,target": {response},
	}}

	checkYAML := `assert:
  generatedResources:
  - metadata:
      name: generated-one
  - metadata:
      name: generated-two
  patchedTarget:
    metadata:
      name: patched-target
  patchedTargetParentResourceGVR:
    group: apps
    version: v1
    resource: deployments
  patchedTargetSubresourceName: scale
`
	var check v1alpha1.CheckResult
	require.NoError(t, yaml.UnmarshalStrict([]byte(checkYAML), &check))

	rc := &resultCounts{}
	var resultsTable table.Table
	require.NoError(t, printCheckResult([]v1alpha1.CheckResult{check}, responses, rc, &resultsTable))
	assert.Equal(t, 1, rc.Pass)
	assert.Equal(t, 0, rc.Fail)
	require.Len(t, resultsTable.RawRows, 1)
}
