package push

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildImageValidCELPolicyAndException(t *testing.T) {
	vp := &policiesv1beta1.ValidatingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-labels",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.labels != null",
			}},
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
				Name: "check-labels",
				Kind: "ValidatingPolicy",
			}},
		},
	}

	results := &policy.LoaderResults{
		ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{vp},
		PolicyCelExceptions: []*policiesv1beta1.PolicyException{polex},
	}

	img, err := buildImage(results)
	require.NoError(t, err)
	require.NotNil(t, img)

	layers, err := img.Layers()
	require.NoError(t, err)
	assert.Len(t, layers, 2)

	manifest, err := img.Manifest()
	require.NoError(t, err)
	assert.Len(t, manifest.Layers, 2)
	assert.Equal(t, "ValidatingPolicy", manifest.Layers[0].Annotations[internal.AnnotationKind])
	assert.Equal(t, "check-labels", manifest.Layers[0].Annotations[internal.AnnotationName])
	assert.Equal(t, "PolicyException", manifest.Layers[1].Annotations[internal.AnnotationKind])
	assert.Equal(t, "exempt-labels", manifest.Layers[1].Annotations[internal.AnnotationName])
}

func TestBuildImageDuplicateIdentity(t *testing.T) {
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

	_, err := buildImage(results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource identity")
}

func TestBuildImageInvalidExceptionReference(t *testing.T) {
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

	_, err := buildImage(results)
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
