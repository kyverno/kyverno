package apply

import (
	"os"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/kyverno/kyverno/pkg/logging"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_Apply_AllowExistingViolations(t *testing.T) {
	logging.Setup(logging.TextFormat, logging.DefaultTime, 4, false)
	policy := `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-latest-tag
spec:
  validationFailureAction: Enforce
  rules:
  - match:
      any:
      - resources:
          kinds:
          - Pod
    name: validate-image-tag
    validate:
      message: Using a mutable image tag e.g. 'latest' is not allowed
      pattern:
        spec:
          containers:
          - image: '!*:latest'
`
	policyWithTrue := `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-latest-tag-true
spec:
  validationFailureAction: Enforce
  rules:
  - match:
      any:
      - resources:
          kinds:
          - Pod
    name: validate-image-tag
    validate:
      allowExistingViolations: true
      message: Using a mutable image tag e.g. 'latest' is not allowed
      pattern:
        spec:
          containers:
          - image: '!*:latest'
`
	policyWithFalse := `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-latest-tag-false
spec:
  validationFailureAction: Enforce
  rules:
  - match:
      any:
      - resources:
          kinds:
          - Pod
    name: validate-image-tag
    validate:
      allowExistingViolations: false
      message: Using a mutable image tag e.g. 'latest' is not allowed
      pattern:
        spec:
          containers:
          - image: '!*:latest'
`
	resource := `
apiVersion: v1
kind: Pod
metadata:
  name: myapp-pod
  labels:
    app: myapp
spec:
  containers:
  - name: nginx
    image: nginx:latest
`

	policyFile, err := createTempFile("policy.yaml", policy)
	assert.NoError(t, err)
	defer os.Remove(policyFile)

	policyFileTrue, err := createTempFile("policy_true.yaml", policyWithTrue)
	assert.NoError(t, err)
	defer os.Remove(policyFileTrue)

	policyFileFalse, err := createTempFile("policy_false.yaml", policyWithFalse)
	assert.NoError(t, err)
	defer os.Remove(policyFileFalse)

	resourceFile, err := createTempFile("resource.yaml", resource)
	assert.NoError(t, err)
	defer os.Remove(resourceFile)

	type TestCase struct {
		expectedReports []openreportsv1alpha1.Report
		config          ApplyCommandConfig
		desc            string
	}

	testcases := []*TestCase{
		{
			desc: "Default behavior (false): should fail on existing violation",
			config: ApplyCommandConfig{
				PolicyPaths:                    []string{policyFile},
				ResourcePaths:                  []string{resourceFile},
				PolicyReport:                   true,
				Variables:                      []string{"request.operation=UPDATE"},
				DefaultAllowExistingViolations: false,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			desc: "Explicit true: should skip/pass on existing violation",
			config: ApplyCommandConfig{
				PolicyPaths:                    []string{policyFile},
				ResourcePaths:                  []string{resourceFile},
				PolicyReport:                   true,
				Variables:                      []string{"request.operation=UPDATE"},
				DefaultAllowExistingViolations: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  0,
					Skip:  1, // It skips because existing violations are allowed
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			desc: "Global false, Rule true: should skip (Rule overrides Global)",
			config: ApplyCommandConfig{
				PolicyPaths:                    []string{policyFileTrue},
				ResourcePaths:                  []string{resourceFile},
				PolicyReport:                   true,
				Variables:                      []string{"request.operation=UPDATE"},
				DefaultAllowExistingViolations: false,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  0,
					Skip:  1,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			desc: "Global true, Rule false: should fail (Rule overrides Global)",
			config: ApplyCommandConfig{
				PolicyPaths:                    []string{policyFileFalse},
				ResourcePaths:                  []string{resourceFile},
				PolicyReport:                   true,
				Variables:                      []string{"request.operation=UPDATE"},
				DefaultAllowExistingViolations: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			_, _, _, responses, err := tc.config.applyCommandHelper(os.Stdout)
			assert.NoError(t, err)

			for _, r := range responses {
				for _, rule := range r.PolicyResponse.Rules {
					t.Logf("Rule: %s, Status: %s, Message: %s", rule.Name(), rule.Status(), rule.Message())
				}
			}

			clustered, _ := report.ComputePolicyReports(tc.config.AuditWarn, responses...)
			combined := []openreportsv1alpha1.ClusterReport{
				report.MergeClusterReports(clustered),
			}
			assert.Equal(t, len(tc.expectedReports), len(combined))
			for i, resp := range combined {
				assert.Equal(t, tc.expectedReports[i].Summary.Pass, resp.Summary.Pass, "Pass count mismatch")
				assert.Equal(t, tc.expectedReports[i].Summary.Fail, resp.Summary.Fail, "Fail count mismatch")
				assert.Equal(t, tc.expectedReports[i].Summary.Skip, resp.Summary.Skip, "Skip count mismatch")
				assert.Equal(t, tc.expectedReports[i].Summary.Error, resp.Summary.Error, "Error count mismatch")
				assert.Equal(t, tc.expectedReports[i].Summary.Warn, resp.Summary.Warn, "Warn count mismatch")
			}
		})
	}
}

func createTempFile(name, content string) (string, error) {
	f, err := os.CreateTemp("", name+"-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
