package push

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// writeTree writes files (bundle-root-relative path -> content) under a fresh temp directory and
// returns its path.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
	return dir
}

// validate is a shorthand for the ported buildImage tests: they exercise only error behavior
// over a hand-built policy.LoaderResults, not the archived-content shape, so a bundle.Bundle
// with no Documents or Files (unpopulated by Assemble) is enough to drive bundle.ValidateWithOptions.
// opts is variadic so every existing call site keeps compiling unchanged; tests that need
// Options (a fixed clock, a warning sink) pass exactly one.
func validate(results *policy.LoaderResults, opts ...bundle.Options) error {
	var o bundle.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return bundle.ValidateWithOptions(&bundle.Bundle{Results: results}, o)
}

func TestPushValidCELPolicyAndExceptionProducesSingleContentLayer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-labels
spec:
  validations:
  - expression: "object.metadata.labels != null"
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exception.yaml"), []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-labels
spec:
  policyRefs:
  - name: check-labels
    kind: ValidatingPolicy
`), 0o600))

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))

	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	require.NotNil(t, img)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1)

	mt, err := layers[0].MediaType()
	require.NoError(t, err)
	assert.Equal(t, internal.ContentMediaType, string(mt))

	manifest, err := img.Manifest()
	require.NoError(t, err)
	require.Len(t, manifest.Layers, 1)
	assert.Equal(t, internal.ConfigMediaType, string(manifest.Config.MediaType))
	assert.Equal(t, "1.0.0", manifest.Annotations[internal.AnnotationFormatVersion])
}

// TestExecuteRejectsVAPWithSpecificMessage drives the real Assemble -> Validate pipeline (not a
// hand-built LoaderResults) so it catches what a unit test over LoaderResults alone cannot:
// validateDocuments' raw per-document pass must not shadow validateResults' more specific
// "native Kubernetes admission policy resources" message with its own generic "unsupported
// resource" one.
func TestExecuteRejectsVAPWithSpecificMessage(t *testing.T) {
	dir := t.TempDir()
	vapYAML := `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: check-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE"]
  validations:
  - expression: "object.metadata.labels != null"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vap.yaml"), []byte(vapYAML), 0o600))

	opts := options{imageRef: "example.com/repo/test:v1"}
	err := opts.execute(context.Background(), dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "native Kubernetes admission policy")
	assert.NotContains(t, err.Error(), "unsupported resource")
}

// TestExecuteRejectsMAPWithSpecificMessage is TestExecuteRejectsVAPWithSpecificMessage's sibling
// for MutatingAdmissionPolicy: the same shadowing bug applied only to the validating half, since
// they're both native admissionregistration.k8s.io kinds checked at the same two sites.
func TestExecuteRejectsMAPWithSpecificMessage(t *testing.T) {
	dir := t.TempDir()
	mapYAML := `apiVersion: admissionregistration.k8s.io/v1alpha1
kind: MutatingAdmissionPolicy
metadata:
  name: add-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE"]
  mutations:
  - patchType: "ApplyConfiguration"
    applyConfiguration:
      expression: "Object{}"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "map.yaml"), []byte(mapYAML), 0o600))

	opts := options{imageRef: "example.com/repo/test:v1"}
	err := opts.execute(context.Background(), dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "native Kubernetes admission policy")
	assert.NotContains(t, err.Error(), "unsupported resource")
}

// TestExceptionReferencingExternalPolicyRoundTripsThroughRead pins the fix for the format-legal
// case the spec explicitly carves out: a PolicyException whose policy reference doesn't resolve
// inside the bundle is legal at the format level (bundle-spec.md section 6, "Sets"); only push's
// own strict default rejects it at write time (writer MUST 12). A bundle push already rejects
// this input; the reader path exercised here is what a bundle built some other way (or built
// once push's rule loosens) must still accept.
func TestExceptionReferencingExternalPolicyRoundTripsThroughRead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exception.yaml"), []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-labels
spec:
  policyRefs:
  - name: some-policy-not-in-this-bundle
    kind: ValidatingPolicy
`), 0o600))

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)

	// Push's own strict default rejects this bundle: the reference doesn't resolve in-bundle.
	err = bundle.Validate(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown policy")

	// But the format itself permits it (writer MUST 12's carve-out), so a reader built from the
	// same content set must accept it: ValidateForRead, which Read uses, must not re-apply
	// push's strict reference-resolution default.
	require.NoError(t, bundle.ValidateForRead(b))

	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	_, err = bundle.Read(img, dst)
	assert.NoError(t, err, "a bundle whose exception references a policy outside the bundle is format-legal and must round-trip through Read")
}

func TestValidateDuplicateIdentity(t *testing.T) {
	vp1 := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
	}
	vp2 := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
	}

	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp1, vp2},
	}

	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource identity")
}

func TestValidateInvalidExceptionReference(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
	}
	polex := &policiesv1beta1.PolicyException{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyException",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "exempt-labels",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "nonexistent-policy",
				Kind: "ValidatingPolicy",
			}},
		},
	}

	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}

	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown policy")
}

func TestExecuteRejectsLegacyKind(t *testing.T) {
	dir := t.TempDir()
	legacyYAML := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-root
spec:
  rules:
  - name: check-root
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Root user is disallowed."
`
	err := os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte(legacyYAML), 0o600)
	require.NoError(t, err)

	opts := options{imageRef: "example.com/repo/test:v1"}
	err = opts.execute(context.Background(), dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1")
}

func TestValidateRejectsNonFatalErrors(t *testing.T) {
	results := &policy.LoaderResults{
		NonFatalErrors: []policy.LoaderError{{
			Path:  "bad.yaml",
			Error: fmt.Errorf("schema validation failed"),
		}},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bundle rejected")
}

func TestValidateRejectsVAPResources(t *testing.T) {
	results := makeResultsWithVAPs()
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bundle rejected")
	assert.Contains(t, err.Error(), "native Kubernetes admission policy")
}

func TestValidateExceptionOnlyBundleAlwaysChecksRefs(t *testing.T) {
	// hasPolicies gate removed: an exception that references a policy not in
	// the same bundle is always a packaging error, even if the bundle contains
	// no policies at all.
	polex := &policiesv1beta1.PolicyException{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyException",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "orphan-exception",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "nonexistent",
				Kind: "ValidatingPolicy",
			}},
		},
	}
	results := &policy.LoaderResults{
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown policy")
}

func makeResultsWithVAPs() *policy.LoaderResults {
	return &policy.LoaderResults{
		VAPs: []admissionregistrationv1.ValidatingAdmissionPolicy{{}},
	}
}

func TestValidateRejectsUnsupportedAPIVersion(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels-alpha",
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource")
	assert.Contains(t, err.Error(), "policies.kyverno.io/v1alpha1")
}

func TestValidateRejectsInvalidExceptionCELExpression(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}
	polex := &policiesv1beta1.PolicyException{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyException",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "exempt-labels",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Kind: "ValidatingPolicy", Name: "check-labels"},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "invalid-condition",
					Expression: "invalid.syntax == (((",
				},
			},
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validating CEL expression in policy exception")
}

func TestValidateJSONModeMutatingPolicy(t *testing.T) {
	newPolicy := func(expression string) *policiesv1beta1.MutatingPolicy {
		expressionJSON, err := json.Marshal(expression)
		require.NoError(t, err)
		var mpol policiesv1beta1.MutatingPolicy
		require.NoError(t, json.Unmarshal([]byte(`{
			"apiVersion":"policies.kyverno.io/v1beta1","kind":"MutatingPolicy","metadata":{"name":"transform"},
			"spec":{"evaluation":{"mode":"JSON"},"mutations":[{"patchType":"JSONPatch","jsonPatch":{"expression":`+string(expressionJSON)+`}}]}
		}`), &mpol))
		return &mpol
	}

	err := validate(&policy.LoaderResults{
		MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{newPolicy(`[JSONPatch{op: "add", path: "/ok", value: true}]`)},
	})
	require.NoError(t, err)
	err = validate(&policy.LoaderResults{
		MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{newPolicy(`[JSONPatch{op: "add", path: "/ok", value: request.object}]`)},
	})
	require.ErrorContains(t, err, "validating CEL expression in MutatingPolicy")
}

func TestValidateRejectsInvalidValidatingPolicyCELExpression(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{Expression: "invalid.syntax == (((", Message: "labels are required"},
			},
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validating CEL expression in ValidatingPolicy")
}

const descriptorPolicyYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validations:
  - expression: "object.metadata.labels != null"
    message: "labels are required"
`

const descriptorExceptionYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-labels
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`

// TestPushWithDescriptorRecordsNamedSetsInConfig drives the whole push pipeline
// (Assemble -> Validate -> LoadDescriptor -> Write) over a descriptor that names both default
// sets explicitly, and checks the resulting config and manifest annotations.
func TestPushWithDescriptorRecordsNamedSetsInConfig(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/require-labels.yaml":  descriptorPolicyYAML,
		"exceptions/exempt-labels.yaml": descriptorExceptionYAML,
		internal.DescriptorFilename: `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  sets:
  - name: policies
    type: policies
    paths: ["policies/**"]
  - name: exceptions
    type: exceptions
    paths: ["exceptions/**"]
`,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)
	require.NotNil(t, d)

	img, err := bundle.Write(b, d)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg bundle.Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))

	assert.ElementsMatch(t, []bundle.ConfigSet{
		{Name: "policies", Type: "policies"},
		{Name: "exceptions", Type: "exceptions"},
	}, cfg.Sets)
	assert.Equal(t, "baseline", cfg.Name)

	bySet := map[string]string{}
	for _, r := range cfg.Resources {
		bySet[r.Name] = r.Set
	}
	assert.Equal(t, "policies", bySet["require-labels"])
	assert.Equal(t, "exceptions", bySet["exempt-labels"])

	manifest, err := img.Manifest()
	require.NoError(t, err)
	assert.Equal(t, "baseline", manifest.Annotations[internal.AnnotationName])
}

// TestPushWithDescriptorOrdersNonReservedSetAfterDefaults pins the order assignSets already
// guarantees (default sets first, by first appearance): a descriptor-declared, non-reserved set
// name appears after the default sets that have members.
func TestPushWithDescriptorOrdersNonReservedSetAfterDefaults(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"team-a/require-labels.yaml": descriptorPolicyYAML,
		"other/other-policy.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: other-policy
spec:
  validations:
  - expression: "true"
`,
		internal.DescriptorFilename: `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  sets:
  - name: team-a
    type: policies
    paths: ["team-a/**"]
`,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)

	img, err := bundle.Write(b, d)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg bundle.Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))

	var names []string
	for _, s := range cfg.Sets {
		names = append(names, s.Name)
	}
	require.Contains(t, names, "policies")
	require.Contains(t, names, "team-a")
	policiesIdx := indexOf(names, "policies")
	teamAIdx := indexOf(names, "team-a")
	assert.Less(t, policiesIdx, teamAIdx, "the default set must appear before the non-reserved descriptor set")
}

// TestPushWithDescriptorNonReservedSetKeepsDeclaredType pins that a non-reserved set's type in the
// config comes from the descriptor, not a hardcoded default: a regression that always wrote
// Type: "policies" would stay green under every other test in this file.
func TestPushWithDescriptorNonReservedSetKeepsDeclaredType(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"team-x/require-labels.yaml": descriptorPolicyYAML,
		"team-x/exempt-labels.yaml":  descriptorExceptionYAML,
		internal.DescriptorFilename: `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  sets:
  - name: team-x
    type: exceptions
    paths: ["team-x/**"]
`,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)

	img, err := bundle.Write(b, d)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg bundle.Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))

	assert.Contains(t, cfg.Sets, bundle.ConfigSet{Name: "team-x", Type: "exceptions"})
	bySet := map[string]string{}
	for _, r := range cfg.Resources {
		bySet[r.Name] = r.Set
	}
	assert.Equal(t, "team-x", bySet["exempt-labels"])
	assert.Equal(t, "policies", bySet["require-labels"], "the policy falls back to the by-kind default; team-x's exceptions type doesn't fit it")
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if s == v {
			return i
		}
	}
	return -1
}

// TestPushWithoutDescriptorIsUnchanged pins that the new LoadDescriptor call is a no-op for a
// tree with no kyverno-bundle.yaml: the config carries only default sets and no name.
func TestPushWithoutDescriptorIsUnchanged(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/require-labels.yaml": descriptorPolicyYAML,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)
	assert.Nil(t, d)

	img, err := bundle.Write(b, d)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg bundle.Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))
	require.Len(t, cfg.Sets, 1)
	assert.Equal(t, "policies", cfg.Sets[0].Name)
	assert.Empty(t, cfg.Name)
}

// TestPushDescriptorRoundTripsThroughRead asserts the descriptor itself is never archived (it's
// excluded at Assemble time) and that Read still succeeds against the named-set config it
// produced.
func TestPushDescriptorRoundTripsThroughRead(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/require-labels.yaml": descriptorPolicyYAML,
		internal.DescriptorFilename: `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  sets:
  - name: policies
    type: policies
    paths: ["policies/**"]
`,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)

	img, err := bundle.Write(b, d)
	require.NoError(t, err)

	dst := t.TempDir()
	got, err := bundle.Read(img, dst)
	require.NoError(t, err)
	require.Len(t, got.Documents, 1)

	_, statErr := os.Stat(filepath.Join(dst, internal.DescriptorFilename))
	assert.True(t, os.IsNotExist(statErr), "the descriptor is never archived, so it must not appear in the extracted tree")
}

// TestPushRejectsDescriptorWithYmlExtensionViaGroupCheck pins the existing message so the spec
// statement that only the exact filename kyverno-bundle.yaml is a descriptor stays true: a
// kyverno-bundle.yml is ordinary content, rejected by Assemble's existing cli.kyverno.io group
// check, not by LoadDescriptor.
func TestPushRejectsDescriptorWithYmlExtensionViaGroupCheck(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/require-labels.yaml": descriptorPolicyYAML,
		"kyverno-bundle.yml": `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
`,
	})

	_, err := bundle.Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cli.kyverno.io")
}

// TestValidateAcceptsV1Resources pins D4: the 1.20 CLI's accepted-version table widens from the
// single v1beta1 pin to v1beta1 and v1, so a v1 ValidatingPolicy must pass where it previously
// would have been rejected as "unsupported resource".
func TestValidateAcceptsV1Resources(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}
	assert.NoError(t, validate(results))
}

// TestValidateWarnsOnExpiredException pins D2 (amended): an expired exception is a warning on the
// strict path, not a hard failure, so push does not refuse to publish content the cluster itself
// would still accept and skip (pkg/cel/engine/exception.go).
func TestValidateWarnsOnExpiredException(t *testing.T) {
	past := metav1.NewTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	polex := &policiesv1beta1.PolicyException{
		TypeMeta:   metav1.TypeMeta{Kind: "PolicyException", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "expired-exception"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt:  &past,
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "check-labels", Kind: "ValidatingPolicy"}},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ValidatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels"},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}

	var warnings []string
	err := validate(results, bundle.Options{
		Now:  func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		Warn: func(msg string) { warnings = append(warnings, msg) },
	})
	assert.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "expired")
}

// TestValidateDoesNotWarnOnUnexpiredException is TestValidateWarnsOnExpiredException's sibling:
// a future expiresAt must not warn, so the injectable clock is pinned on both sides.
func TestValidateDoesNotWarnOnUnexpiredException(t *testing.T) {
	future := metav1.NewTime(time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC))
	polex := &policiesv1beta1.PolicyException{
		TypeMeta:   metav1.TypeMeta{Kind: "PolicyException", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "live-exception"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt:  &future,
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "check-labels", Kind: "ValidatingPolicy"}},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ValidatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels"},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}

	var warnings []string
	err := validate(results, bundle.Options{
		Now:  func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		Warn: func(msg string) { warnings = append(warnings, msg) },
	})
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

// TestValidateRejectsMalformedSelector pins D3: an offline-checkable malformed selector on a
// policy's matchConstraints fails strict validation (push, validate), the same gate as reference
// resolution.
func TestValidateRejectsMalformedSelector(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ValidatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "bad-selector"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "env", Operator: "NotAnOperator", Values: []string{"prod"}},
					},
				},
			},
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespaceSelector")
}

// TestValidateRejectsUncompilableCompensatingControl pins D7: a PolicyException compensating
// control whose CEL fails to compile is a hard failure on the strict path, the same call pattern
// pkg/webhooks/celexception/validate.go already uses for admission.
func TestValidateRejectsUncompilableCompensatingControl(t *testing.T) {
	polex := &policiesv1beta1.PolicyException{
		TypeMeta:   metav1.TypeMeta{Kind: "PolicyException", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "bad-control"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "check-labels", Kind: "ValidatingPolicy"}},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "invalid.syntax == ((("},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ValidatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels"},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compensating control")
}

// TestValidateRejectsMalformedObjectSelector is TestValidateRejectsMalformedSelector's sibling for
// objectSelector, the other half of checkSelectors' pair of checks.
func TestValidateRejectsMalformedObjectSelector(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ValidatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "bad-object-selector"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "env", Operator: "NotAnOperator", Values: []string{"prod"}},
					},
				},
			},
		},
	}
	results := &policy.LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}
	err := validate(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "objectSelector")
}

// TestValidateWarnsOnCompensatingControlForIgnoringKind pins the warning half of D7:
// ValidateCompensatingControls, not ValidateCompensatingControlExpressions, fires when an
// exception's compensating control references a kind other than ValidatingPolicy or
// NamespacedValidatingPolicy, whose engine ignores spec.validations entirely.
func TestValidateWarnsOnCompensatingControlForIgnoringKind(t *testing.T) {
	polex := &policiesv1beta1.PolicyException{
		TypeMeta:   metav1.TypeMeta{Kind: "PolicyException", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "ignored-control"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "mutate-things", Kind: "MutatingPolicy"}},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}
	mp := &policiesv1beta1.MutatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-things"},
	}
	results := &policy.LoaderResults{
		MutatingPolicies:    []policiesv1beta1.MutatingPolicyLike{mp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}

	var warnings []string
	err := validate(results, bundle.Options{Warn: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "ignored-control")
	assert.Contains(t, warnings[0], "MutatingPolicy")
}
