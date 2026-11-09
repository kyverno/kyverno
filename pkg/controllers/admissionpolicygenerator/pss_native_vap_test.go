package admissionpolicygenerator

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

// TestPSSNativeVAP exercises every built-in PSS Helm ValidatingPolicy through
// the same controller that creates the Kubernetes-native VAP and binding.
// The opt-in must preserve Pod validation and match criteria, while the default
// must continue to auto-generate rules for workload Pod templates.
func TestPSSNativeVAP(t *testing.T) {
	helm := os.Getenv("HELM")
	if helm == "" {
		helm = "helm"
	}
	if _, err := exec.LookPath(helm); err != nil {
		t.Skipf("Helm is needed to render the PSS chart: %v", err)
	}

	chart := filepath.Join("..", "..", "..", "charts", "kyverno-policies")
	profiles := []struct {
		name  string
		count int
	}{
		{name: "baseline", count: 12},
		{name: "restricted", count: 19},
	}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			defaultPolicies := renderPSSNativeVAPPolicies(t, helm, chart, profile.name)
			require.Len(t, defaultPolicies, profile.count)
			for _, policy := range defaultPolicies {
				// Default PSS behavior validates workload Pod templates as well.
				require.False(t, policy.Spec.GenerateValidatingAdmissionPolicyEnabled(), policy.Name)
				configs, err := vpolautogen.Autogen(&policy)
				require.NoError(t, err, policy.Name)
				require.NotEmpty(t, configs, policy.Name)
			}

			policies := renderPSSNativeVAPPolicies(t, helm, chart, profile.name,
				"--set", "generateValidatingAdmissionPolicy=true",
				"--set", "failurePolicy=Ignore")
			require.Len(t, policies, profile.count)
			for _, policy := range policies {
				t.Run(policy.Name, func(t *testing.T) {
					require.True(t, policy.Spec.GenerateValidatingAdmissionPolicyEnabled())
					require.NotNil(t, policy.Spec.AutogenConfiguration)
					require.NotNil(t, policy.Spec.AutogenConfiguration.PodControllers)
					require.NotNil(t, policy.Spec.AutogenConfiguration.PodControllers.Enabled)
					assert.False(t, *policy.Spec.AutogenConfiguration.PodControllers.Enabled)
					configs, err := vpolautogen.Autogen(&policy)
					require.NoError(t, err)
					assert.Empty(t, configs, "native VAPs cannot include Pod-controller rules")
					assert.NotEmpty(t, policy.Spec.Validations)

					ctx := context.Background()
					c, kubeClient, kyvernoClient := newVAPTestController(t, &policy)
					require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(&policy)))

					nativeName := "vpol-" + policy.Name
					vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, nativeName, metav1.GetOptions{})
					require.NoError(t, err)
					binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, nativeName+"-binding", metav1.GetOptions{})
					require.NoError(t, err)

					// The API-server policy must execute the exact same CEL rules.
					assert.Equal(t, policy.Spec.MatchConstraints, vap.Spec.MatchConstraints)
					assert.Equal(t, policy.Spec.MatchConditions, vap.Spec.MatchConditions)
					assert.Equal(t, policy.Spec.Variables, vap.Spec.Variables)
					assert.Equal(t, policy.Spec.Validations, vap.Spec.Validations)
					assert.Equal(t, policy.Spec.AuditAnnotations, vap.Spec.AuditAnnotations)
					assert.Equal(t, policy.Spec.FailurePolicy, vap.Spec.FailurePolicy)
					assert.Equal(t, nativeName, binding.Spec.PolicyName)
					assert.Equal(t, policy.Spec.ValidationActions(), binding.Spec.ValidationActions)
					require.Len(t, vap.OwnerReferences, 1)
					require.Len(t, binding.OwnerReferences, 1)
					assert.Equal(t, policy.Name, vap.OwnerReferences[0].Name)
					assert.Equal(t, policy.Name, binding.OwnerReferences[0].Name)

					updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
					require.NoError(t, err)
					assert.True(t, updated.Status.Generated)
				})
			}
		})
	}

	t.Run("service-account-exclusions-use-native-CEL", func(t *testing.T) {
		policies := renderPSSNativeVAPPolicies(t, helm, chart, "baseline",
			"--set", "generateValidatingAdmissionPolicy=true",
			"--set", "vpolExclude.excludeSubjects[0].kind=ServiceAccount",
			"--set", "vpolExclude.excludeSubjects[0].namespace=kube-system",
			"--set", "vpolExclude.excludeSubjects[0].name=default")
		require.Len(t, policies, 12)
		for _, policy := range policies {
			require.NotEmpty(t, policy.Spec.MatchConditions, policy.Name)
			found := false
			for _, condition := range policy.Spec.MatchConditions {
				assert.NotContains(t, condition.Expression, "parseServiceAccount(", policy.Name)
				if strings.Contains(condition.Expression, "system:serviceaccount:kube-system:default") {
					found = true
				}
			}
			assert.True(t, found, "missing native-compatible ServiceAccount exclusion for %s", policy.Name)
		}
	})
}

func renderPSSNativeVAPPolicies(t *testing.T, helm, chart, profile string, args ...string) []policiesv1beta1.ValidatingPolicy {
	t.Helper()
	helmArgs := []string{"template", "verify-pss", chart, "--kube-version", "v1.37.0", "--set", "podSecurityStandard=" + profile, "--set", "podSecurityVersion=v1.37"}
	helmArgs = append(helmArgs, args...)
	cmd := exec.Command(helm, helmArgs...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "helm template failed: %s", string(output))

	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(output), 4096)
	policies := make([]policiesv1beta1.ValidatingPolicy, 0)
	for {
		var policy policiesv1beta1.ValidatingPolicy
		err := decoder.Decode(&policy)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if policy.Kind == "" {
			continue
		}
		require.Equal(t, "ValidatingPolicy", policy.Kind)
		policies = append(policies, policy)
	}
	return policies
}
