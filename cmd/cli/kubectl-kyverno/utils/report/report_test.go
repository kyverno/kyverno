package report

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var rawPolicy = []byte(`
{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "pod-requirements",
	  "annotations": {
		"pod-policies.kyverno.io/autogen-controllers": "none",
		"policies.kyverno.io/severity": "medium",
		"policies.kyverno.io/category": "Pod Security Standards (Restricted)"
	  }
	},
	"spec": {
	  "background": false,
	  "validationFailureAction": "audit",
	  "rules": [
		{
		  "name": "pods-require-account",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "User pods must include an account for charging",
			"pattern": {
			  "metadata": {
				"labels": {
				  "account": "*?"
				}
			  }
			}
		  }
		},
		{
		  "name": "pods-require-limits",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "CPU and memory resource requests and limits are required for user pods",
			"pattern": {
			  "spec": {
				"containers": [
				  {
					"resources": {
					  "requests": {
						"memory": "?*",
						"cpu": "?*"
					  },
					  "limits": {
						"memory": "?*",
						"cpu": "?*"
					  }
					}
				  }
				]
			  }
			}
		  }
		}
	  ]
	}
  }
`)

func TestComputePolicyReports(t *testing.T) {
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(&policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{},
		*engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
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
		assert.Equal(t, report.Results[0].Severity, policyreportv1alpha2.SeverityMedium)
		assert.Equal(t, report.Results[0].Category, "Pod Security Standards (Restricted)")
		assert.Equal(t, report.Summary.Pass, 1)
	}
}

func TestComputePolicyReportResultsPerPolicy(t *testing.T) {
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	er := engineapi.EngineResponse{}
	er = er.WithPolicy(engineapi.NewKyvernoPolicy(&policy))
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{}, *engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
		),
	)
	results := ComputePolicyReportResultsPerPolicy(false, er)
	for _, result := range results {
		assert.Equal(t, len(result), 2)
		for _, r := range result {
			switch r.Rule {
			case "pods-require-limits":
				assert.Equal(t, r.Result, policyreportv1alpha2.StatusPass)
			case "pods-require-account":
				assert.Equal(t, r.Result, policyreportv1alpha2.StatusFail)
			}
		}
	}
}

func TestMergeClusterReport(t *testing.T) {
	clustered := []policyreportv1alpha2.ClusterPolicyReport{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: report.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpolr-4",
		},
		Results: []policyreportv1alpha2.PolicyReportResult{
			{
				Policy: "cpolr-4",
				Result: report.StatusFail,
			},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: report.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpolr-5",
		},
		Results: []policyreportv1alpha2.PolicyReportResult{
			{
				Policy: "cpolr-5",
				Result: report.StatusFail,
			},
		},
	}}
	namespaced := []policyreportv1alpha2.PolicyReport{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyReport",
			APIVersion: report.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-polr-1",
			Namespace: "ns-polr",
		},
		Results: []policyreportv1alpha2.PolicyReportResult{
			{
				Policy:    "ns-polr-1",
				Result:    report.StatusPass,
				Resources: make([]corev1.ObjectReference, 10),
			},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyReport",
			APIVersion: report.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns-polr-2",
		},
		Results: []policyreportv1alpha2.PolicyReportResult{
			{
				Policy:    "ns-polr-2",
				Result:    report.StatusPass,
				Resources: make([]corev1.ObjectReference, 5),
			},
		},
	}, {
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyReport",
			APIVersion: report.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "polr-3",
		},
		Results: []policyreportv1alpha2.PolicyReportResult{
			{
				Policy:    "polr-3",
				Result:    report.StatusPass,
				Resources: make([]corev1.ObjectReference, 1),
			},
		},
	}}
	expectedResults := []policyreportv1alpha2.PolicyReportResult{{
		Policy: "cpolr-4",
		Result: report.StatusFail,
	}, {
		Policy: "cpolr-5",
		Result: report.StatusFail,
	}, {
		Policy:    "ns-polr-2",
		Result:    report.StatusPass,
		Resources: make([]corev1.ObjectReference, 5),
	}, {
		Policy:    "polr-3",
		Result:    report.StatusPass,
		Resources: make([]corev1.ObjectReference, 1),
	}}
	cpolr := MergeClusterReports(clustered, namespaced)
	assert.Equal(t, cpolr.APIVersion, report.SchemeGroupVersion.String())
	assert.Equal(t, cpolr.Kind, "ClusterPolicyReport")
	assert.DeepEqual(t, cpolr.Results, expectedResults)
	assert.Equal(t, cpolr.Summary.Pass, 2)
	assert.Equal(t, cpolr.Summary.Fail, 2)
}
