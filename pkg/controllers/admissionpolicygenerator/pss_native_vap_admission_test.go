package admissionpolicygenerator

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	celmatching "github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
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

// pssViolatingPod creates a targeted failure case for every chart policy.
// Each case starts from the same Restricted-compliant Pod used by the allow path.
func pssViolatingPod(t *testing.T, policyName, validPod string) string {
	t.Helper()
	var pod map[string]any
	require.NoError(t, json.Unmarshal([]byte(validPod), &pod))
	spec := pod["spec"].(map[string]any)
	podSecurity := spec["securityContext"].(map[string]any)
	container := spec["containers"].([]any)[0].(map[string]any)
	containerSecurity := container["securityContext"].(map[string]any)

	switch policyName {
	case "disallow-capabilities":
		containerSecurity["capabilities"] = map[string]any{"add": []string{"SYS_ADMIN"}}
	case "disallow-host-namespaces":
		spec["hostPID"] = true
	case "disallow-host-path", "restrict-volume-types":
		spec["volumes"] = []any{map[string]any{"name": "host", "hostPath": map[string]any{"path": "/tmp"}}}
	case "disallow-host-ports":
		container["ports"] = []any{map[string]any{"containerPort": 80, "hostPort": 8080}}
	case "disallow-host-probes":
		container["livenessProbe"] = map[string]any{"httpGet": map[string]any{"host": "example.com", "path": "/", "port": 80}}
	case "disallow-host-process":
		containerSecurity["windowsOptions"] = map[string]any{"hostProcess": true}
	case "disallow-privileged-containers":
		containerSecurity["privileged"] = true
	case "disallow-proc-mount", "disallow-proc-mount-strict":
		containerSecurity["procMount"] = "Unmasked"
	case "disallow-selinux":
		containerSecurity["seLinuxOptions"] = map[string]any{"user": "root"}
	case "restrict-apparmor-profiles":
		containerSecurity["appArmorProfile"] = map[string]any{"type": "Unconfined"}
	case "restrict-seccomp":
		containerSecurity["seccompProfile"] = map[string]any{"type": "Unconfined"}
	case "restrict-sysctls":
		podSecurity["sysctls"] = []any{map[string]any{"name": "kernel.kptr_restrict", "value": "0"}}
	case "disallow-capabilities-strict":
		containerSecurity["capabilities"] = map[string]any{"drop": []string{}}
	case "disallow-privilege-escalation":
		containerSecurity["allowPrivilegeEscalation"] = true
	case "require-run-as-non-root-user":
		podSecurity["runAsUser"] = 0
	case "require-run-as-nonroot":
		podSecurity["runAsNonRoot"] = false
	case "restrict-seccomp-strict":
		podSecurity["seccompProfile"] = map[string]any{"type": "Unconfined"}
		containerSecurity["seccompProfile"] = map[string]any{"type": "Unconfined"}
	default:
		t.Fatalf("no native admission-denial fixture for %q", policyName)
	}
	b, err := json.Marshal(pod)
	require.NoError(t, err)
	return string(b)
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

// verifyPSSNativeSubresourceUpdateMatch exercises Kubernetes admission
// resource-rule matching with the actual UPDATE operation and subresource,
// rather than relying only on the rendered resource names.
func verifyPSSNativeSubresourceUpdateMatch(t *testing.T, policy *admissionregistrationv1.ValidatingAdmissionPolicy, rawPod string) {
	t.Helper()
	var pod unstructured.Unstructured
	require.NoError(t, json.Unmarshal([]byte(rawPod), &pod.Object))
	attrs := admission.NewAttributesRecord(
		&pod, nil, pod.GroupVersionKind(), pod.GetNamespace(), pod.GetName(),
		schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		"ephemeralcontainers", admission.Update, &metav1.UpdateOptions{}, false,
		&user.DefaultInfo{Name: "pss-test-user"},
	)
	matched, err := celmatching.NewMatcher().Match(
		&celmatching.MatchCriteria{Constraints: policy.Spec.MatchConstraints},
		attrs,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: pod.GetNamespace()}},
	)
	require.NoError(t, err, "resource-rule matching failed for %s", policy.Name)
	assert.True(t, matched, "native VAP must match pods/ephemeralcontainers UPDATE")
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
	validEphemeralPod := pssAdmissionPod(safeSecurity, safeSecurity)

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
					assert.Equal(t, engineapi.RuleStatusPass, evaluatePSSNativeCEL(t, vap, binding, validPod), "PSS-compliant Pod must pass validation")
					verifyPSSNativeSubresourceUpdateMatch(t, vap, validEphemeralPod)
					assert.Equal(t, engineapi.RuleStatusPass, evaluatePSSNativeCEL(t, vap, binding, validEphemeralPod), "compliant ephemeral container must pass validation")

					// A positive-only check would not detect a vacuous always-true
					// translation. Each PSS policy must reject its own violation.
					assert.Equal(t, engineapi.RuleStatusFail,
						evaluatePSSNativeCEL(t, vap, binding, pssViolatingPod(t, pol.Name, validPod)),
						"generated VAP must reject its policy-specific violation")

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
