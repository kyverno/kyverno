package admissionpolicygenerator

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
)

// A JSON mode policy has no Kubernetes admission equivalent, so no native
// MutatingAdmissionPolicy is generated for it even when generation is requested.
func TestMapGenerationSkipReason_JSONMode(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration:    mapGenEnabled(),
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
	}
	reason, err := mapGenerationSkipReason(policy)
	assert.NoError(t, err)
	assert.Equal(t, "skip generating MutatingAdmissionPolicy: JSON evaluation mode has no admission equivalent.", reason)

	policy.Spec.EvaluationConfiguration.Mode = policieskyvernoio.EvaluationModeKubernetes
	reason, err = mapGenerationSkipReason(policy)
	assert.NoError(t, err)
	assert.Empty(t, reason)
}

// Switching a policy with a generated MutatingAdmissionPolicy to JSON mode
// deletes the generated MAP and binding and clears status.generated, so the
// policy is no longer enforced at admission.
func TestHandleMAPGeneration_JSONModeDeletesGeneratedMAP(t *testing.T) {
	ctx := context.Background()
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration:    mapGenEnabled(),
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
		Status: policiesv1beta1.MutatingPolicyStatus{Generated: true},
	}
	mapName := "mpol-" + mpol.Name
	bindingName := constructBindingName(mapName)
	existingMAP := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: mapName, ResourceVersion: "1"}}
	existingBinding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName, ResourceVersion: "1"}}

	kubeClient := kubefake.NewSimpleClientset(existingMAP, existingBinding)
	kyvernoClient := versionedfake.NewSimpleClientset(mpol)
	mapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, mapIndexer.Add(existingMAP))
	require.NoError(t, bindingIndexer.Add(existingBinding))
	c := &controller{
		client:             kubeClient,
		kyvernoClient:      kyvernoClient,
		checker:            permissiveAuthChecker{},
		mapV1Lister:        admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapIndexer),
		mapbindingV1Lister: admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	require.NoError(t, c.handleMAPGeneration(ctx, mpol))

	_, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, mapName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "generated MAP must be deleted")
	_, err = kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "generated MAP binding must be deleted")
	updated, err := kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(ctx, mpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "JSON evaluation mode")
}
