package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

type mockPolexListerForSuccess struct{}

func (m *mockPolexListerForSuccess) List(selector labels.Selector) ([]*kyvernov2.PolicyException, error) {
	return nil, nil
}
func (m *mockPolexListerForSuccess) PolicyExceptions(namespace string) kyvernov2listers.PolicyExceptionNamespaceLister {
	return nil
}

type mockDiscoveryForError struct {
	dclient.IDiscovery
	err error
}

func (m *mockDiscoveryForError) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	return nil, m.err
}

func TestHandleVAPGeneration_SentinelError(t *testing.T) {
	errSentinel := errors.New("sentinel vap list error")
	c := &controller{
		client:         k8sfake.NewSimpleClientset(),
		kyvernoClient:  fake.NewSimpleClientset(),
		vapLister:      &mockVAPListerForError{err: errSentinel},
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

// removed mockCelPolexListerForSuccess from here, it is in generate_map_test.go

func TestHandleVAPGeneration_APIError(t *testing.T) {
	errSentinel := errors.New("sentinel vap api error")
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("create", "validatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errSentinel
	})
	c := &controller{
		client:           client,
		kyvernoClient:    fake.NewSimpleClientset(),
		vapLister:        &mockVAPListerForError{err: apierrors.NewNotFound(schema.GroupResource{}, "notfound")},
		vapbindingLister: &mockVAPBindingLister{},
		celpolexLister:   &mockCelPolexListerForSuccess{},
		checker:          &mockAuthChecker{},
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

func TestHandleVAPGeneration_BuildError(t *testing.T) {
	errSentinel := errors.New("sentinel vap build error")
	c := &controller{
		client:           k8sfake.NewSimpleClientset(),
		kyvernoClient:    fake.NewSimpleClientset(),
		vapLister:        &mockVAPListerForError{err: apierrors.NewNotFound(schema.GroupResource{}, "notfound")},
		vapbindingLister: &mockVAPBindingLister{},
		celpolexLister:   &mockCelPolexListerForSuccess{},
		polexLister:      &mockPolexListerForSuccess{},
		checker:          &mockAuthChecker{},
		discoveryClient:  &mockDiscoveryForError{err: errSentinel},
	}

	cpol := &kyvernov1.ClusterPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "ClusterPolicy", APIVersion: "kyverno.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cpol"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					MatchResources: kyvernov1.MatchResources{
						Any: kyvernov1.ResourceFilters{
							{
								ResourceDescription: kyvernov1.ResourceDescription{
									Kinds: []string{"Pod"},
								},
							},
						},
					},
					Validation: &kyvernov1.Validation{
						CEL: &kyvernov1.CEL{
							Generate:    ptr.To(true),
							Expressions: []admissionregistrationv1.Validation{},
						},
					},
				},
			},
		},
	}
	policy := api.NewKyvernoPolicy(cpol)
	err := c.handleVAPGeneration(context.Background(), "ClusterPolicy", policy)
	assert.ErrorIs(t, err, errSentinel)
}
