package test

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

// TestCompileGeneratingPolicies_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the
// namespace argument compileGeneratingPolicies passes into exception selection: a namespaced
// generating policy must only compile in exceptions filed in its own namespace, never a
// same-named policy's exception from a foreign namespace. It fails if compileGeneratingPolicies
// ever goes back to passing the unfiltered exception slice straight into compiler.Compile.
func TestCompileGeneratingPolicies_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"},
	}
	ngpol.TypeMeta.Kind = "NamespacedGeneratingPolicy"

	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}

	compiled, err := compileGeneratingPolicies(
		[]policiesv1beta1.GeneratingPolicyLike{ngpol},
		[]*policiesv1beta1.PolicyException{ownException, foreignException},
	)
	require.NoError(t, err)
	require.Len(t, compiled, 1)

	res := unstructured.Unstructured{}
	res.SetGroupVersionKind(schema.GroupVersionKind{})
	res.SetName("trigger")
	res.SetNamespace("tenant-a")

	attr := admission.NewAttributesRecord(
		&res, &res, schema.GroupVersionKind{}, res.GetNamespace(), res.GetName(),
		schema.GroupVersionResource{}, "", admission.Connect, &res, false, &user.DefaultInfo{},
	)
	req := admissionv1.AdmissionRequest{
		UID:       "test",
		Namespace: res.GetNamespace(),
		UserInfo:  authenticationv1.UserInfo{},
	}

	result, err := compiled[0].CompiledPolicy.Evaluate(context.TODO(), attr, &req, &res, &libs.FakeContextProvider{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", result.Exceptions[0].GetNamespace())
}
