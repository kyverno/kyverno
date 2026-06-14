package admissionpolicy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// ---- minimal fake engine client ----

type fakeEngineClient struct{}

func (f *fakeEngineClient) IsNamespaced(group, version, kind string) (bool, error) {
	return true, nil
}

func (f *fakeEngineClient) GetResource(
	ctx context.Context,
	apiVersion, kind, namespace, name, subresource string,
) (runtime.Object, error) {
	return &admissionregistrationv1.MutatingAdmissionPolicy{}, nil
}

func (f *fakeEngineClient) ListResource(
	ctx context.Context,
	apiVersion, kind, namespace string,
	selector interface{},
) (*admissionregistrationv1.MutatingAdmissionPolicyList, error) {
	return &admissionregistrationv1.MutatingAdmissionPolicyList{}, nil
}

// ---- tests ----

func TestHasValidatingAdmissionPolicyPermission(t *testing.T) {
	var checker interface{} = nil
	// function should safely handle nil checker without panic
	assert.False(t, HasValidatingAdmissionPolicyPermission(checker.(interface {
	})))
}

func TestPreferredMutatingAdmissionPolicyVersion_NotRegistered(t *testing.T) {
	client := kubernetes.NewForConfigOrDie(nil)

	_, err := PreferredMutatingAdmissionPolicyVersion(client)
	assert.Error(t, err)
}

func TestIsValidatingAdmissionPolicyRegistered(t *testing.T) {
	client := kubernetes.NewForConfigOrDie(nil)

	ok, err := IsValidatingAdmissionPolicyRegistered(client)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestCollectParams_InvalidGroupVersion(t *testing.T) {
	client := &fakeEngineClient{}

	kind := &admissionregistrationv1.ParamKind{
		APIVersion: "invalid-version",
		Kind:       "Dummy",
	}

	ref := &admissionregistrationv1.ParamRef{}

	_, err := CollectParams(
		context.TODO(),
		client,
		kind,
		ref,
		"default",
	)

	assert.Error(t, err)
}

func TestGroupVersionParsing(t *testing.T) {
	_, err := schema.ParseGroupVersion("admissionregistration.k8s.io/v1")
	assert.NoError(t, err)
}
