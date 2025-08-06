package report

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

func TestComputeClusterReports(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/cpol-pod-requirements.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{},
		*engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
			nil,
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
			nil,
		),
	)
	clustered, namespaced := ComputePolicyReports(false, er)
	assert.Equal(t, len(clustered), 1)
	assert.Equal(t, len(namespaced), 0)
	{
		report := clustered[0]
		assert.Equal(t, report.GetName(), policy.GetName())
		assert.Equal(t, report.Kind, "ClusterReport")
		assert.Equal(t, len(report.Results), 2)
		assert.Equal(t, report.Results[0].Severity, openreportsv1alpha1.ResultSeverity(openreports.SeverityMedium))
		assert.Equal(t, report.Results[0].Category, "Pod Security Standards (Restricted)")
		assert.Equal(t, report.Summary.Pass, 1)
	}
}

func TestComputePolicyReports(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/pol-pod-requirements.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{},
		*engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
			nil,
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
			nil,
		),
	)
	clustered, namespaced := ComputePolicyReports(false, er)
	assert.Equal(t, len(clustered), 0)
	assert.Equal(t, len(namespaced), 1)
	{
		report := namespaced[0]
		assert.Equal(t, report.GetName(), policy.GetName())
		assert.Equal(t, report.GetNamespace(), policy.GetNamespace())
		assert.Equal(t, report.Kind, "PolicyReport")
		assert.Equal(t, len(report.Results), 2)
		assert.Equal(t, report.Results[0].Severity, openreportsv1alpha1.ResultSeverity(openreports.SeverityMedium))
		assert.Equal(t, report.Results[0].Category, "Pod Security Standards (Restricted)")
		assert.Equal(t, report.Summary.Pass, 1)
	}
}

func TestComputeReportResultsPerPolicyOld(t *testing.T) {
	loaderResults, err := policy.Load(nil, "", "../_testdata/policies/cpol-pod-requirements.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(loaderResults.Policies), 1)
	policy := loaderResults.Policies[0]
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{}, *engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
			nil,
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
			nil,
		),
	)
	results := ComputePolicyReportResultsPerPolicy(false, er)
	for _, result := range results {
		assert.Equal(t, len(result), 2)
		for _, r := range result {
			switch r.Rule {
			case "pods-require-limits":
				assert.Equal(t, r.Result, openreportsv1alpha1.Result(openreports.StatusPass))
			case "pods-require-account":
				assert.Equal(t, r.Result, openreportsv1alpha1.Result(openreports.StatusFail))
			}
		}
	}
}

func TestMergeClusterReport(t *testing.T) {
	clustered := []openreportsv1alpha1.ClusterReport{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterReport",
			APIVersion: openreportsv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpolr-4",
		},
		Results: []openreportsv1alpha1.ReportResult{
			{
				Policy: "cpolr-4",
				Result: openreports.StatusFail,
			},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterReport",
			APIVersion: openreportsv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpolr-5",
		},
		Results: []openreportsv1alpha1.ReportResult{
			{
				Policy: "cpolr-5",
				Result: openreports.StatusFail,
			},
		},
	}}
	expectedResults := []openreportsv1alpha1.ReportResult{{
		Policy: "cpolr-4",
		Result: openreports.StatusFail,
	}, {
		Policy: "cpolr-5",
		Result: openreports.StatusFail,
	}}
	cpolr := MergeClusterReports(clustered)
	assert.Equal(t, cpolr.APIVersion, openreportsv1alpha1.SchemeGroupVersion.String())
	assert.Equal(t, cpolr.Kind, "ClusterReport")
	assert.DeepEqual(t, cpolr.Results, expectedResults)
	assert.Equal(t, cpolr.Summary.Pass, 0)
	assert.Equal(t, cpolr.Summary.Fail, 2)
}

func TestComputeReportResult(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/cpol-pod-requirements.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	tests := []struct {
		name           string
		auditWarn      bool
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           openreportsv1alpha1.ReportResult
	}{{
		name:           "skip",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleSkip("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusSkip,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}, {
		name:           "pass",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RulePass("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusPass,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}, {
		name:           "fail",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusFail,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}, {
		name:           "fail - audit warn",
		auditWarn:      true,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusWarn,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}, {
		name:           "error",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleError("xxx", engineapi.Mutation, "test", nil, nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusError,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}, {
		name:           "warn",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleWarn("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      openreports.StatusWarn,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    openreports.SeverityMedium,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResult(tt.auditWarn, tt.engineResponse, tt.ruleResponse)
			got.Timestamp = metav1.Timestamp{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeReportResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPSSComputeReportResult(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/restricted.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	tests := []struct {
		name           string
		auditWarn      bool
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           openreportsv1alpha1.ReportResult
	}{{
		name:           "fail",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "psa",
			Rule:        "xxx",
			Result:      openreports.StatusFail,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Severity:    openreports.SeverityMedium,
			Properties:  map[string]string{"process": "background scan"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResult(tt.auditWarn, tt.engineResponse, tt.ruleResponse)
			got.Timestamp = metav1.Timestamp{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeReportResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeReportResultsPerPolicy(t *testing.T) {
	tests := []struct {
		name            string
		auditWarn       bool
		engineResponses []engineapi.EngineResponse
		want            map[engineapi.GenericPolicy][]openreportsv1alpha1.ReportResult
	}{{
		name:      "empty",
		auditWarn: false,
		engineResponses: func() []engineapi.EngineResponse {
			return []engineapi.EngineResponse{{}}
		}(),
		want: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResultsPerPolicy(tt.auditWarn, tt.engineResponses...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeReportResultsPerPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamespacedPolicyReportGeneration(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/namespace-policy.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]

	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{},
		*engineapi.RuleFail(
			"validate-pod",
			engineapi.Validation,
			"validation error: Pods must have a label `app`.",
			nil,
		),
	)

	clustered, namespaced := ComputePolicyReports(false, er)

	assert.Equal(t, len(clustered), 0)
	assert.Equal(t, len(namespaced), 1)

	report := namespaced[0]
	assert.Equal(t, report.GetNamespace(), policy.GetNamespace())
	assert.Equal(t, report.Kind, "PolicyReport")
}
