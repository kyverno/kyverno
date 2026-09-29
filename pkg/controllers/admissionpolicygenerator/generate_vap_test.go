package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
)

type mockVAPListerForError struct{ err error }

type mockVAPBindingLister struct{}

func (m *mockVAPBindingLister) List(selector labels.Selector) ([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	return nil, nil
}
func (m *mockVAPBindingLister) Get(name string) (*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "notfound")
}

func (m *mockVAPListerForError) List(selector labels.Selector) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	return nil, m.err
}
func (m *mockVAPListerForError) Get(name string) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	return nil, m.err
}

type mockCelPolexNamespaceListerForError struct{ err error }

func (m *mockCelPolexNamespaceListerForError) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, m.err
}
func (m *mockCelPolexNamespaceListerForError) Get(name string) (*policiesv1beta1.PolicyException, error) {
	return nil, m.err
}

type mockCelPolexListerForError struct{ err error }

func (m *mockCelPolexListerForError) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, m.err
}
func (m *mockCelPolexListerForError) PolicyExceptions(namespace string) policiesv1beta1listers.PolicyExceptionNamespaceLister {
	return &mockCelPolexNamespaceListerForError{err: m.err}
}



func TestHandleVAPGeneration_SentinelError(t *testing.T) {
	errSentinel := errors.New("sentinel vap list error")
	c := &controller{
		client:         k8sfake.NewSimpleClientset(),
		kyvernoClient:  fake.NewSimpleClientset(),
		vapLister:      &mockVAPListerForError{err: errSentinel},
		celpolexLister: &mockCelPolexListerForError{err: errSentinel},
		checker:        &mockAuthChecker{},
	}

	mpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				ValidatingAdmissionPolicy: &policiesv1beta1.VapGenerationConfiguration{Enabled: ptr.To(true)},
			},
		},
	}
	err := c.handleVAPGeneration(context.Background(), "test-policy", api.NewValidatingPolicy(mpol))
	assert.ErrorIs(t, err, errSentinel)
}

// removed mockCelPolexListerForSuccess from here, it is in generate_map_test.go

func TestHandleVAPGeneration_APIError(t *testing.T) {
	errSentinel := errors.New("sentinel vap api error")
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("create", "validatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errSentinel
	})
	c := &controller{
		client:         client,
		kyvernoClient:  fake.NewSimpleClientset(),
		vapLister:      &mockVAPListerForError{err: apierrors.NewNotFound(schema.GroupResource{}, "notfound")},
		vapbindingLister: &mockVAPBindingLister{},
		celpolexLister: &mockCelPolexListerForSuccess{},
		checker:        &mockAuthChecker{},
	}

	mpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				ValidatingAdmissionPolicy: &policiesv1beta1.VapGenerationConfiguration{Enabled: ptr.To(true)},
			},
		},
	}
	err := c.handleVAPGeneration(context.Background(), "test-policy", api.NewValidatingPolicy(mpol))
	assert.ErrorIs(t, err, errSentinel)
}
