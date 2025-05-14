package report

import (
	"reflect"
	"testing"

	reportv1alpha1 "github.com/kyverno/kyverno/api/openreports.io/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestComputeClusterPolicyReports(t *testing.T) {
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
		assert.Equal(t, report.Kind, "ClusterPolicyReport")
		assert.Equal(t, len(report.Results), 2)
		assert.Equal(t, report.Results[0].Severity, reportv1alpha1.SeverityMedium)
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
		assert.Equal(t, report.Results[0].Severity, reportv1alpha1.SeverityMedium)
		assert.Equal(t, report.Results[0].Category, "Pod Security Standards (Restricted)")
		assert.Equal(t, report.Summary.Pass, 1)
	}
}

func TestComputePolicyReportResultsPerPolicyOld(t *testing.T) {
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
				assert.Equal(t, r.Result, reportv1alpha1.StatusPass)
			case "pods-require-account":
				assert.Equal(t, r.Result, reportv1alpha1.StatusFail)
			}
		}
	}
}

func TestMergeClusterReport(t *testing.T) {
	clustered := []reportv1alpha1.ClusterReport{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterReport",
			APIVersion: reportv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cepr-4",
		},
		Results: []reportv1alpha1.ReportResult{
			{
				Policy: "cepr-4",
				Result: reportv1alpha1.StatusFail,
			},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterReport",
			APIVersion: reportv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpolr-5",
		},
		Results: []reportv1alpha1.ReportResult{
			{
				Policy: "cpolr-5",
				Result: reportv1alpha1.StatusFail,
			},
		},
	}}
	expectedResults := []reportv1alpha1.ReportResult{{
		Policy: "cpolr-4",
		Result: reportv1alpha1.StatusFail,
	}, {
		Policy: "cpolr-5",
		Result: reportv1alpha1.StatusFail,
	}}
	cpolr := MergeClusterReports(clustered)
	assert.Equal(t, cpolr.APIVersion, reportv1alpha1.SchemeGroupVersion.String())
	assert.Equal(t, cpolr.Kind, "ClusterPolicyReport")
	assert.DeepEqual(t, cpolr.Results, expectedResults)
	assert.Equal(t, cpolr.Summary.Pass, 0)
	assert.Equal(t, cpolr.Summary.Fail, 2)
}

func TestComputePolicyReportResult(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/cpol-pod-requirements.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	tests := []struct {
		name           string
		auditWarn      bool
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           reportv1alpha1.ReportResult
	}{{
		name:           "skip",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleSkip("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusSkip,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}, {
		name:           "pass",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RulePass("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusPass,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}, {
		name:           "fail",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusFail,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}, {
		name:           "fail - audit warn",
		auditWarn:      true,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusWarn,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}, {
		name:           "error",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleError("xxx", engineapi.Mutation, "test", nil, nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusError,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}, {
		name:           "warn",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleWarn("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "pod-requirements",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusWarn,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Properties:  map[string]string{"process": "admission review"},
			Severity:    reportv1alpha1.SeverityMedium,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResult(tt.auditWarn, tt.engineResponse, tt.ruleResponse)
			got.Timestamp = metav1.Timestamp{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputePolicyReportResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPSSComputePolicyReportResult(t *testing.T) {
	results, err := policy.Load(nil, "", "../_testdata/policies/restricted.yaml")
	assert.NilError(t, err)
	assert.Equal(t, len(results.Policies), 1)
	policy := results.Policies[0]
	tests := []struct {
		name           string
		auditWarn      bool
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           reportv1alpha1.ReportResult
	}{{
		name:           "fail",
		auditWarn:      false,
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewKyvernoPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: reportv1alpha1.ReportResult{
			Source:      "kyverno",
			Policy:      "psa",
			Rule:        "xxx",
			Result:      reportv1alpha1.StatusFail,
			Subjects:    []corev1.ObjectReference{{}},
			Description: "test",
			Scored:      true,
			Category:    "Pod Security Standards (Restricted)",
			Severity:    reportv1alpha1.SeverityMedium,
			Properties:  map[string]string{"process": "background scan"},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResult(tt.auditWarn, tt.engineResponse, tt.ruleResponse)
			got.Timestamp = metav1.Timestamp{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputePolicyReportResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputePolicyReportResultsPerPolicy(t *testing.T) {
	tests := []struct {
		name            string
		auditWarn       bool
		engineResponses []engineapi.EngineResponse
		want            map[engineapi.GenericPolicy][]reportv1alpha1.ReportResult
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
				t.Errorf("ComputePolicyReportResultsPerPolicy() = %v, want %v", got, tt.want)
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
