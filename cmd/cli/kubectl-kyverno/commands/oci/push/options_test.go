package push

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

// validate is a shorthand for the ported buildImage tests: they exercise only error behavior
// over a hand-built policy.LoaderResults, not the archived-content shape, so a bundle.Bundle
// with no Documents or Files (unpopulated by Assemble) is enough to drive bundle.Validate.
func validate(results *policy.LoaderResults) error {
	return bundle.Validate(&bundle.Bundle{Results: results})
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
