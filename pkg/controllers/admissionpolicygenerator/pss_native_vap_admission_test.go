package admissionpolicygenerator

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// pssAdmissionPod builds a Pod with the supplied container and optional ephemeral
// container security contexts. The baseline object satisfies the Restricted PSS.
func pssAdmissionPod(containerSecurity, ephemeralSecurity string) string {
	ephemeral := ""
	if ephemeralSecurity != "" {
		ephemeral = `,"ephemeralContainers":[{"name":"debug","image":"busybox","securityContext":` + ephemeralSecurity + `}]`
	}
	return `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pss-native-admission","namespace":"default"},"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":1000,"runAsGroup":1000,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"main","image":"busybox","securityContext":` + containerSecurity + `}]` + ephemeral + `}}`
}

// evaluatePSSNativeCEL uses Kubernetes' validating-admission-policy compiler
// and validator rather than comparing CEL strings or using a fake policy API.
// This is an in-process CEL check, NOT a substitute for an API-server test of
// the pods/ephemeralcontainers UPDATE admission endpoint.
func evaluatePSSNativeCEL(t *testing.T, policy *admissionregistrationv1.ValidatingAdmissionPolicy, binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding, rawPod string) engineapi.RuleStatus {
	t.Helper()
	var pod unstructured.Unstructured
	require.NoError(t, json.Unmarshal([]byte(rawPod), &pod.Object))

	policyData := engineapi.NewValidatingAdmissionPolicyData(policy)
	policyData.AddBinding(*binding)
	result, err := admissionpolicy.Validate(
		policyData,
		pod,
		pod.GroupVersionKind(),
		schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		map[string]map[string]string{},
		nil,
		&authenticationv1.UserInfo{Username: "pss-test-user"},
		true,
	)
	require.NoError(t, err)
	require.Len(t, result.PolicyResponse.Rules, 1, "expected exactly one CEL decision for %s", policy.Name)
	return result.PolicyResponse.Rules[0].Status()
}

func TestPSSNativeVAPAdmissionDecisions(t *testing.T) {
	helm := os.Getenv("HELM")
	if helm == "" {
		helm = "helm"
	}
	if _, err := exec.LookPath(helm); err != nil {
		t.Skipf("Helm is needed to render the PSS chart: %v", err)
	}
	chart := filepath.Join("..", "..", "..", "charts", "kyverno-policies")
	safeSecurity := `{"privileged":false,"allowPrivilegeEscalation":false,"procMount":"Default","capabilities":{"drop":["ALL"]},"runAsNonRoot":true,"runAsUser":1000,"runAsGroup":1000,"seccompProfile":{"type":"RuntimeDefault"},"appArmorProfile":{"type":"RuntimeDefault"}}`
	badPrivileged := `{"privileged":true,"allowPrivilegeEscalation":false,"procMount":"Default","capabilities":{"drop":["ALL"]}}`
	badEscalation := `{"privileged":false,"allowPrivilegeEscalation":true,"procMount":"Default","capabilities":{"drop":["ALL"]}}`
	badProcMount := `{"privileged":false,"allowPrivilegeEscalation":false,"procMount":"Unmasked","capabilities":{"drop":["ALL"]}}`
	validPod := pssAdmissionPod(safeSecurity, "")

	type admissionCase struct {
		name string
		pod  string
	}
	violations := map[string][]admissionCase{
		"disallow-privileged-containers": {
			{"privileged regular container", pssAdmissionPod(badPrivileged, "")},
			{"privileged ephemeral container", pssAdmissionPod(safeSecurity, badPrivileged)},
		},
		"disallow-privilege-escalation": {
			{"privilege escalation regular container", pssAdmissionPod(badEscalation, "")},
			{"privilege escalation ephemeral container", pssAdmissionPod(safeSecurity, badEscalation)},
		},
		"disallow-proc-mount-strict": {
			{"unmasked proc mount regular container", pssAdmissionPod(badProcMount, "")},
			{"unmasked proc mount ephemeral container", pssAdmissionPod(safeSecurity, badProcMount)},
		},
	}

	for _, profile := range []struct {
		name         string
		args         []string
		wantPolicies int
	}{
		{name: "baseline", wantPolicies: 12},
		{name: "restricted", wantPolicies: 18},
		{name: "restricted-user-namespaces", wantPolicies: 19, args: []string{"--set", "podSecurityUserNamespaces=true"}},
	} {
		t.Run(profile.name, func(t *testing.T) {
			profileName := profile.name
			if profileName == "restricted-user-namespaces" {
				profileName = "restricted"
			}
			helmArgs := []string{"--set", "generateValidatingAdmissionPolicy=true", "--set", "validationFailureAction=Enforce"}
			helmArgs = append(helmArgs, profile.args...)
			policies := renderPSSNativeVAPPolicies(t, helm, chart, profileName, helmArgs...)
			require.Len(t, policies, profile.wantPolicies)
			for _, pol := range policies {
				t.Run(pol.Name, func(t *testing.T) {
					pol.UID = "pss-native-admission-test"
					ctx := context.Background()
					c, client, _ := newVAPTestController(t, &pol)
					require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(&pol)))
					nativeName := "vpol-" + pol.Name
					vap, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, nativeName, metav1.GetOptions{})
					require.NoError(t, err)
					binding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, nativeName+"-binding", metav1.GetOptions{})
					require.NoError(t, err)
					require.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, binding.Spec.ValidationActions)
					status := evaluatePSSNativeCEL(t, vap, binding, validPod)
					assert.NotEqual(t, engineapi.RuleStatusFail, status, "PSS-compliant Pod should not be denied")
					assert.NotEqual(t, engineapi.RuleStatusError, status, "PSS-compliant Pod must compile and evaluate without errors")

					for _, tc := range violations[pol.Name] {
						t.Run(tc.name, func(t *testing.T) {
							assert.Equal(t, engineapi.RuleStatusFail, evaluatePSSNativeCEL(t, vap, binding, tc.pod))
						})
					}
				})
			}
		})
	}
}
