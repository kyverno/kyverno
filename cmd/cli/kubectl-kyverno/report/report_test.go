package report

import (
	"encoding/json"
	"reflect"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// pssAnnotations mirrors the policies.kyverno.io/category and policies.kyverno.io/severity
// annotations the retired legacy fixtures carried, so report metadata stays comparable.
func pssAnnotations() map[string]string {
	return map[string]string{
		kyverno.AnnotationPolicyCategory: "Pod Security Standards (Restricted)",
		kyverno.AnnotationPolicySeverity: "medium",
	}
}

// admissionOnlyVpol builds a cluster-scoped ValidatingPolicy fixture that runs on admission only,
// matching the admission:true/background:false legacy fixtures this test used to load from YAML.
func admissionOnlyVpol(name, namespace string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: pssAnnotations(),
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Admission:  &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(true)},
				Background: &policiesv1beta1.BackgroundConfiguration{Enabled: ptr.To(false)},
			},
		},
	}
}

func TestComputePolicyReportsDecode(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NilError(t, openreportsv1alpha1.Install(scheme))
	decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	for _, tc := range []struct {
		name      string
		namespace string
		kind      string
	}{
		{"cluster", "", "ClusterReport"},
		{"namespaced", "test", "Report"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := admissionOnlyVpol("pod-requirements", tc.namespace)
			response := engineapi.EngineResponse{}.WithPolicy(engineapi.NewValidatingPolicy(p))
			response.PolicyResponse.Add(engineapi.ExecutionStats{}, *engineapi.RulePass("check", engineapi.Validation, "passed", nil))
			clustered, namespaced := ComputePolicyReports(false, response)
			assert.Equal(t, len(clustered)+len(namespaced), 1)
			var report interface{}
			if len(clustered) == 1 {
				report = clustered[0]
			} else {
				report = namespaced[0]
			}
			for _, format := range []struct {
				name    string
				marshal func(interface{}) ([]byte, error)
			}{
				{"json", json.Marshal},
				{"yaml", yaml.Marshal},
			} {
				t.Run(format.name, func(t *testing.T) {
					data, err := format.marshal(report)
					assert.NilError(t, err)
					decoded, gvk, err := decoder.Decode(data, nil, nil)
					assert.NilError(t, err)
					assert.Equal(t, *gvk, openreportsv1alpha1.SchemeGroupVersion.WithKind(tc.kind))
					assert.Equal(t, decoded.(metav1.Object).GetName(), p.GetName())
					assert.Equal(t, decoded.(metav1.Object).GetNamespace(), p.GetNamespace())
				})
			}
		})
	}
}

func TestComputeClusterReports(t *testing.T) {
	policy := admissionOnlyVpol("pod-requirements", "")
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewValidatingPolicy(policy))
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
	policy := admissionOnlyVpol("pod-requirements", "test")
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewValidatingPolicy(policy))
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
		assert.Equal(t, report.Kind, "Report")
		assert.Equal(t, len(report.Results), 2)
		assert.Equal(t, report.Results[0].Severity, openreportsv1alpha1.ResultSeverity(openreports.SeverityMedium))
		assert.Equal(t, report.Results[0].Category, "Pod Security Standards (Restricted)")
		assert.Equal(t, report.Summary.Pass, 1)
	}
}

func TestComputeReportResultsPerPolicyOld(t *testing.T) {
	policy := admissionOnlyVpol("pod-requirements", "")
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewValidatingPolicy(policy))
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
	policy := admissionOnlyVpol("pod-requirements", "")
	tests := []struct {
		name           string
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           openreportsv1alpha1.ReportResult
	}{{
		name:           "skip",
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleSkip("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RulePass("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
		name:           "error",
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleError("xxx", engineapi.Mutation, "test", nil, nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleWarn("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
			got := ComputePolicyReportResult(false, tt.engineResponse, tt.ruleResponse)
			got.Timestamp = metav1.Timestamp{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeReportResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPSSComputeReportResult(t *testing.T) {
	// background-only fixture (background:true, no explicit admission override), matching the
	// retired restricted.yaml legacy fixture.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "psa",
			Annotations: pssAnnotations(),
		},
	}
	tests := []struct {
		name           string
		engineResponse engineapi.EngineResponse
		ruleResponse   engineapi.RuleResponse
		want           openreportsv1alpha1.ReportResult
	}{{
		name:           "fail",
		engineResponse: engineapi.NewEngineResponse(unstructured.Unstructured{}, engineapi.NewValidatingPolicy(policy), nil),
		ruleResponse:   *engineapi.RuleFail("xxx", engineapi.Mutation, "test", nil),
		want: openreportsv1alpha1.ReportResult{
			Source:      "KyvernoValidatingPolicy",
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
			got := ComputePolicyReportResult(false, tt.engineResponse, tt.ruleResponse)
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
		engineResponses []engineapi.EngineResponse
		want            map[engineapi.GenericPolicy][]openreportsv1alpha1.ReportResult
	}{{
		name: "empty",
		engineResponses: func() []engineapi.EngineResponse {
			return []engineapi.EngineResponse{{}}
		}(),
		want: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputePolicyReportResultsPerPolicy(false, tt.engineResponses...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputeReportResultsPerPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamespacedPolicyReportGeneration(t *testing.T) {
	policy := admissionOnlyVpol("require-namespace-labels", "test")

	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewValidatingPolicy(policy))
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
	assert.Equal(t, report.Kind, "Report")
}
