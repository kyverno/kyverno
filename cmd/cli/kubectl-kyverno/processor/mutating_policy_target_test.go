package processor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const targetFixture = "../../../../test/cli/test-mutating-policy/mutate-existing-secret-trigger-deployment-target"

// readUnstructured reads a single resource from a YAML file.
func readUnstructured(t *testing.T, path string) *unstructured.Unstructured {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	var object map[string]any
	require.NoError(t, yaml.Unmarshal(content, &object))
	return &unstructured.Unstructured{Object: object}
}

// TestApplyPoliciesOnResource_MutatingPolicyTargets checks the trigger and target passes for a
// MutatingPolicy with targetMatchConstraints: the trigger is never mutated, the target is mutated only
// when the trigger matches the policy, a failed target mutation is counted as an error, and only the
// patched target is printed or written to the output file.
func TestApplyPoliciesOnResource_MutatingPolicyTargets(t *testing.T) {
	loaded, err := policy.Load(nil, "", false, filepath.Join(targetFixture, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, loaded.MutatingPolicies, 1)

	content, err := os.ReadFile(filepath.Join(targetFixture, "policy.yaml"))
	require.NoError(t, err)
	failingPath := filepath.Join(t.TempDir(), "failing-policy.yaml")
	require.NoError(t, os.WriteFile(failingPath, []byte(strings.Replace(string(content), "replicas: 3", "replicas: object.spec.missing.count", 1)), 0o600))
	failing, err := policy.Load(nil, "", false, failingPath)
	require.NoError(t, err)
	require.Len(t, failing.MutatingPolicies, 1)

	// the same policy with admission disabled: background only policies still mutate their targets
	backgroundOnlyPath := filepath.Join(t.TempDir(), "background-only-policy.yaml")
	backgroundOnly := strings.Replace(string(content), "    mutateExisting:\n      enabled: true", "    admission:\n      enabled: false\n    mutateExisting:\n      enabled: true", 1)
	require.NotEqual(t, string(content), backgroundOnly)
	require.NoError(t, os.WriteFile(backgroundOnlyPath, []byte(backgroundOnly), 0o600))
	background, err := policy.Load(nil, "", false, backgroundOnlyPath)
	require.NoError(t, err)
	require.Len(t, background.MutatingPolicies, 1)
	require.False(t, background.MutatingPolicies[0].GetSpec().AdmissionEnabled())

	secret := readUnstructured(t, filepath.Join(targetFixture, "trigger.yaml"))
	configMap := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "test-config", "namespace": "test-ns"},
	}}
	target := readUnstructured(t, filepath.Join(targetFixture, "target.yaml"))

	tests := []struct {
		name           string
		failing        bool
		backgroundOnly bool
		trigger        *unstructured.Unstructured
		mutateLogPath  string // "-" means a file that cannot be opened
		wantPass       int
		wantError      int
		wantErr        string
	}{
		{name: "matching trigger mutates the target", trigger: secret, wantPass: 1},
		{name: "trigger the policy does not match leaves the target alone", trigger: configMap},
		{name: "failed target mutation is counted as an error", failing: true, trigger: secret, wantError: 1},
		{name: "background only policy still mutates the target", backgroundOnly: true, trigger: secret, wantPass: 1},
		{name: "background only policy still ignores a trigger it does not match", backgroundOnly: true, trigger: configMap},
		{name: "patched target is written to the output file", trigger: secret, mutateLogPath: "file", wantPass: 1},
		{name: "unwritable output file fails the run", trigger: secret, mutateLogPath: "-", wantErr: "failed to print mutated target result"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mpols := loaded.MutatingPolicies
			if tt.failing {
				mpols = failing.MutatingPolicies
			}
			if tt.backgroundOnly {
				mpols = background.MutatingPolicies
			}
			var mutateLogPath string
			switch tt.mutateLogPath {
			case "file":
				mutateLogPath = filepath.Join(t.TempDir(), "mutated.yaml")
				require.NoError(t, os.WriteFile(mutateLogPath, nil, 0o600))
			case "-":
				mutateLogPath = filepath.Join(t.TempDir(), "missing-dir", "mutated.yaml")
			}
			rc := &ResultCounts{}
			out := &bytes.Buffer{}
			processor := PolicyProcessor{
				Store:              &store.Store{},
				MutatingPolicies:   mpols,
				Resource:           *tt.trigger.DeepCopy(),
				TargetResources:    []*unstructured.Unstructured{target.DeepCopy()},
				Variables:          &variables.Variables{},
				Rc:                 rc,
				MutateLogPath:      mutateLogPath,
				PrintPatchResource: true,
				Out:                out,
			}
			responses, err := processor.ApplyPoliciesOnResource()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPass, rc.Pass)
			assert.Equal(t, tt.wantError, rc.Error)

			for _, response := range responses {
				assert.Equal(t, "Deployment", response.Resource.GetKind(), "the trigger must never be evaluated by a target only policy")
				for _, rule := range response.PolicyResponse.Rules {
					patched, _, _ := rule.PatchedTarget()
					if rule.Status() == engineapi.RuleStatusPass {
						require.NotNil(t, patched)
						assert.Equal(t, "true", patched.GetLabels()["mutated"])
					} else {
						assert.Nil(t, patched, "a failed mutation must not produce a patched target")
					}
				}
			}
			if tt.trigger == configMap {
				assert.Empty(t, responses, "a trigger the policy does not match must not mutate any target")
			}

			printed := out.String()
			if mutateLogPath != "" {
				written, err := os.ReadFile(mutateLogPath)
				require.NoError(t, err)
				printed = string(written)
				assert.True(t, strings.HasPrefix(printed, "apiVersion: "), "unexpected start of output: %q", printed)
			}
			assert.Equal(t, tt.wantPass > 0, strings.Contains(printed, "name: test-deploy"))
			assert.NotContains(t, printed, "kind: Secret", "the trigger must not be printed as mutated")
		})
	}
}

// TestApplyPoliciesOnResource_InlineMutatingPolicyOutput checks that an ordinary MutatingPolicy, which
// has no targetMatchConstraints, still mutates the trigger inline and prints or writes it.
func TestApplyPoliciesOnResource_InlineMutatingPolicyOutput(t *testing.T) {
	const fixture = "../../../../test/cli/test-mutating-policy/mutating-label"
	loaded, err := policy.Load(nil, "", false, filepath.Join(fixture, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, loaded.MutatingPolicies, 1)
	resource := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "dev-deploy-2", "namespace": "dev"},
		"spec":       map[string]any{"replicas": int64(1)},
	}}

	for _, toFile := range []bool{false, true} {
		t.Run(map[bool]string{false: "stdout", true: "output file"}[toFile], func(t *testing.T) {
			var mutateLogPath string
			if toFile {
				mutateLogPath = filepath.Join(t.TempDir(), "mutated.yaml")
				require.NoError(t, os.WriteFile(mutateLogPath, nil, 0o600))
			}
			rc := &ResultCounts{}
			out := &bytes.Buffer{}
			processor := PolicyProcessor{
				Store:              &store.Store{},
				MutatingPolicies:   loaded.MutatingPolicies,
				Resource:           *resource.DeepCopy(),
				Variables:          &variables.Variables{},
				Rc:                 rc,
				MutateLogPath:      mutateLogPath,
				PrintPatchResource: true,
				Out:                out,
			}
			_, err := processor.ApplyPoliciesOnResource()
			require.NoError(t, err)
			// the inline path counts its response more than once on main as well, only check it passed
			assert.NotZero(t, rc.Pass)
			assert.Zero(t, rc.Fail+rc.Error)

			printed := out.String()
			if toFile {
				written, err := os.ReadFile(mutateLogPath)
				require.NoError(t, err)
				printed = string(written)
				assert.True(t, strings.HasPrefix(printed, "apiVersion: "), "unexpected start of output: %q", printed)
			}
			assert.Contains(t, printed, "name: dev-deploy-2")
			assert.NotContains(t, printed, "patched targets")
		})
	}
}

// loadMutatingPolicies writes the given manifest to a file and loads the MutatingPolicies in it.
func loadMutatingPolicies(t *testing.T, manifest string) policy.LoaderResults {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policies.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))
	loaded, err := policy.Load(nil, "", false, path)
	require.NoError(t, err)
	return *loaded
}

// namespacedTargetPolicy is the Secret trigger, Deployment target policy as a NamespacedMutatingPolicy in
// the given namespace.
func namespacedTargetPolicy(namespace string) string {
	return `apiVersion: policies.kyverno.io/v1
kind: NamespacedMutatingPolicy
metadata:
  name: namespaced-target-policy
  namespace: ` + namespace + `
spec:
  evaluation:
    mutateExisting:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["secrets"]
  targetMatchConstraints:
    resourceRules:
    - apiGroups: ["apps"]
      apiVersions: ["v1"]
      operations: ["CREATE", "UPDATE"]
      resources: ["deployments"]
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: Object.metadata.labels{mutated: "true"}}}
`
}

// TestApplyPoliciesOnResource_NamespacedMutatingPolicyTargetScope checks that a namespaced policy only
// mutates targets for a trigger in its own namespace, like the admission matcher.
func TestApplyPoliciesOnResource_NamespacedMutatingPolicyTargetScope(t *testing.T) {
	secret := readUnstructured(t, filepath.Join(targetFixture, "trigger.yaml"))
	target := readUnstructured(t, filepath.Join(targetFixture, "target.yaml"))
	require.Equal(t, "test-ns", secret.GetNamespace())

	for _, tt := range []struct {
		namespace string
		wantPass  int
	}{
		{namespace: "test-ns", wantPass: 1},
		{namespace: "other-ns"},
	} {
		t.Run("policy in "+tt.namespace, func(t *testing.T) {
			loaded := loadMutatingPolicies(t, namespacedTargetPolicy(tt.namespace))
			require.Len(t, loaded.MutatingPolicies, 1)
			rc := &ResultCounts{}
			processor := PolicyProcessor{
				Store:              &store.Store{},
				MutatingPolicies:   loaded.MutatingPolicies,
				Resource:           *secret.DeepCopy(),
				TargetResources:    []*unstructured.Unstructured{target.DeepCopy()},
				Variables:          &variables.Variables{},
				Rc:                 rc,
				PrintPatchResource: true,
				Out:                &bytes.Buffer{},
			}
			responses, err := processor.ApplyPoliciesOnResource()
			require.NoError(t, err)
			assert.Equal(t, tt.wantPass, rc.Pass)
			assert.Len(t, responses, tt.wantPass, "a policy from another namespace must not mutate targets of this trigger")
		})
	}
}

// celTargetPolicy is a policy named same-name whose CEL target expression selects one ConfigMap and labels
// it with owner, as a MutatingPolicy when namespace is empty and as a NamespacedMutatingPolicy otherwise.
func celTargetPolicy(namespace, configMap, owner string) string {
	header := "apiVersion: policies.kyverno.io/v1beta1\nkind: MutatingPolicy\nmetadata:\n  name: same-name\n"
	// a namespaced policy reads resources from its own namespace only, so it names no namespace
	get := `resource.get("v1", "configmaps", object.metadata.namespace, "` + configMap + `")`
	if namespace != "" {
		header = "apiVersion: policies.kyverno.io/v1\nkind: NamespacedMutatingPolicy\nmetadata:\n  name: same-name\n  namespace: " + namespace + "\n"
		get = `resource.get("v1", "configmaps", "` + configMap + `")`
	}
	return header + `spec:
  evaluation:
    mutateExisting:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["secrets"]
  targetMatchConstraints:
    expression: >
      ` + get + `
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: Object.metadata.labels{owner: "` + owner + `"}}}
`
}

// TestApplyPoliciesOnResource_CELTargetsKeyedByPolicyKey checks that a clustered and a namespaced policy
// with the same name each mutate only the target their own CEL expression selected.
func TestApplyPoliciesOnResource_CELTargetsKeyedByPolicyKey(t *testing.T) {
	secret := readUnstructured(t, filepath.Join(targetFixture, "trigger.yaml"))
	configMap := func(name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": name, "namespace": "test-ns"},
		}}
	}
	loaded := loadMutatingPolicies(t, celTargetPolicy("", "cm-cluster", "cluster")+"---\n"+celTargetPolicy("test-ns", "cm-namespaced", "namespaced"))
	require.Len(t, loaded.MutatingPolicies, 2)

	rc := &ResultCounts{}
	processor := PolicyProcessor{
		Store:              &store.Store{},
		MutatingPolicies:   loaded.MutatingPolicies,
		Resource:           *secret.DeepCopy(),
		TargetResources:    []*unstructured.Unstructured{configMap("cm-cluster"), configMap("cm-namespaced")},
		Variables:          &variables.Variables{},
		Rc:                 rc,
		PrintPatchResource: true,
		Out:                &bytes.Buffer{},
	}
	responses, err := processor.ApplyPoliciesOnResource()
	require.NoError(t, err)

	owners := map[string][]string{}
	for _, response := range responses {
		for _, rule := range response.PolicyResponse.Rules {
			patched, _, _ := rule.PatchedTarget()
			require.NotNil(t, patched, "rule %q: %s", rule.Name(), rule.Message())
			owners[patched.GetName()] = append(owners[patched.GetName()], patched.GetLabels()["owner"])
		}
	}
	assert.Equal(t, map[string][]string{"cm-cluster": {"cluster"}, "cm-namespaced": {"namespaced"}}, owners)
	assert.Equal(t, 2, rc.Pass)
}
