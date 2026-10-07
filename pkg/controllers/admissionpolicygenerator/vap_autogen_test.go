package admissionpolicygenerator

import (
	"context"
	"errors"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestAutogenVAPName(t *testing.T) {
	assert.Equal(t, "autogen-vpol-check-pods-defaults", autogenVAPName("check-pods", "defaults"))
	assert.Equal(t, "autogen-vpol-check-pods-cronjobs", autogenVAPName("check-pods", "cronjobs"))
	assert.Equal(t, "autogen-vpol-check-pods-my-key", autogenVAPName("check-pods", "My_Key"))

	long := strings.Repeat("a", 250)
	name := autogenVAPName(long, "defaults")
	assert.LessOrEqual(t, len(constructBindingName(name)), validation.DNS1123SubdomainMaxLength)
	assert.Empty(t, validation.IsDNS1123Subdomain(name))
	assert.Empty(t, validation.IsDNS1123Subdomain(constructBindingName(name)))
	assert.Equal(t, name, autogenVAPName(long, "defaults"), "name must be stable")
	assert.NotEqual(t, name, autogenVAPName(long, "cronjobs"), "different keys must not collide")
}

func TestPruneStaleAutogenVAPs(t *testing.T) {
	pol := &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "check-pods", UID: "test-uid"}}
	owned := map[string]string{autogenSourceLabel: labelValue(pol.Name)}
	vap := func(name string, lbls map[string]string, ownerUID string) *admissionregistrationv1.ValidatingAdmissionPolicy {
		vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls}}
		if ownerUID != "" {
			vap.OwnerReferences = []metav1.OwnerReference{{UID: types.UID(ownerUID)}}
		}
		return vap
	}
	binding := func(name, policyName string, lbls map[string]string, ownerUID string) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
		binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls},
			Spec:       admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: policyName},
		}
		if ownerUID != "" {
			binding.OwnerReferences = []metav1.OwnerReference{{UID: types.UID(ownerUID)}}
		}
		return binding
	}
	active := autogenVAPName(pol.Name, "defaults")
	stale := autogenVAPName(pol.Name, "cronjobs")
	objects := []any{
		vap(active, owned, "test-uid"),
		binding(constructBindingName(active), active, owned, "test-uid"),
		vap(stale, owned, "test-uid"),
		binding(constructBindingName(stale), stale, owned, "test-uid"),
		// orphaned binding whose VAP is already gone; its name does not follow the convention
		binding("orphan", "autogen-vpol-check-pods-old", owned, "test-uid"),
		// resources owned by another policy must never be touched
		vap("autogen-vpol-other-defaults", map[string]string{autogenSourceLabel: labelValue("other")}, "other-uid"),
	}

	newController := func() (*controller, *fake.Clientset) {
		vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		client := fake.NewClientset()
		for _, obj := range objects {
			switch o := obj.(type) {
			case *admissionregistrationv1.ValidatingAdmissionPolicy:
				assert.NoError(t, vapIndexer.Add(o))
				_, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(context.TODO(), o, metav1.CreateOptions{})
				assert.NoError(t, err)
			case *admissionregistrationv1.ValidatingAdmissionPolicyBinding:
				assert.NoError(t, bindingIndexer.Add(o))
				_, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(context.TODO(), o, metav1.CreateOptions{})
				assert.NoError(t, err)
			}
		}
		return &controller{
			client:           client,
			vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
			vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer),
		}, client
	}
	names := func(client *fake.Clientset) ([]string, []string) {
		vaps, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		bindings, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		var vapNames, bindingNames []string
		for _, v := range vaps.Items {
			vapNames = append(vapNames, v.Name)
		}
		for _, b := range bindings.Items {
			bindingNames = append(bindingNames, b.Name)
		}
		return vapNames, bindingNames
	}

	t.Run("keeps active and removes stale", func(t *testing.T) {
		c, client := newController()
		assert.NoError(t, c.pruneStaleAutogenVAPs(context.TODO(), pol, map[string]struct{}{active: {}}))
		vapNames, bindingNames := names(client)
		assert.ElementsMatch(t, []string{active, "autogen-vpol-other-defaults"}, vapNames)
		assert.ElementsMatch(t, []string{constructBindingName(active)}, bindingNames)
	})

	t.Run("nil active set removes everything owned by the policy", func(t *testing.T) {
		c, client := newController()
		assert.NoError(t, c.deleteAutogenVAPs(context.TODO(), pol))
		vapNames, bindingNames := names(client)
		assert.ElementsMatch(t, []string{"autogen-vpol-other-defaults"}, vapNames)
		assert.Empty(t, bindingNames)
	})

	t.Run("ignores NotFound error on VAP delete", func(t *testing.T) {
		c, client := newController()
		client.PrependReactor("delete", "validatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{}, action.(k8stesting.DeleteAction).GetName())
		})
		assert.NoError(t, c.pruneStaleAutogenVAPs(context.TODO(), pol, map[string]struct{}{active: {}}))
	})

	t.Run("propagates non-NotFound error on VAP delete", func(t *testing.T) {
		c, client := newController()
		expectedErr := errors.New("delete failed")
		client.PrependReactor("delete", "validatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, expectedErr
		})
		err := c.pruneStaleAutogenVAPs(context.TODO(), pol, map[string]struct{}{active: {}})
		assert.ErrorContains(t, err, "failed to delete stale autogen validatingadmissionpolicy")
	})

	t.Run("ignores NotFound error on VAPBinding delete", func(t *testing.T) {
		c, client := newController()
		client.PrependReactor("delete", "validatingadmissionpolicybindings", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{}, action.(k8stesting.DeleteAction).GetName())
		})
		assert.NoError(t, c.pruneStaleAutogenVAPs(context.TODO(), pol, map[string]struct{}{active: {}}))
	})

	t.Run("propagates non-NotFound error on VAPBinding delete", func(t *testing.T) {
		c, client := newController()
		expectedErr := errors.New("delete binding failed")
		client.PrependReactor("delete", "validatingadmissionpolicybindings", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, expectedErr
		})
		err := c.pruneStaleAutogenVAPs(context.TODO(), pol, map[string]struct{}{active: {}})
		assert.ErrorContains(t, err, "failed to delete stale autogen validatingadmissionpolicybinding")
	})
}

func TestReconcileAutogenVAP_Update(t *testing.T) {
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.replicas > 0"},
			},
		},
	}

	vapName := "autogen-vpol-test-policy-defaults"
	bindingName := constructBindingName(vapName)

	existingVAP := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: vapName, ResourceVersion: "1"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{Expression: "old expression"},
			},
		},
	}

	existingBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName, ResourceVersion: "1"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "old-policy",
		},
	}

	client := fake.NewClientset(existingVAP, existingBinding)
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, vapIndexer.Add(existingVAP))
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	assert.NoError(t, bindingIndexer.Add(existingBinding))

	c := &controller{
		client:           client,
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	autogenConfig := policiesv1beta1.ValidatingPolicyAutogen{
		Spec: &pol.Spec,
	}

	err := c.reconcileAutogenVAP(context.TODO(), pol, vapName, "defaults", autogenConfig, nil)
	assert.NoError(t, err)

	updatedVAP, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.TODO(), vapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "object.spec.replicas > 0", updatedVAP.Spec.Validations[0].Expression)
	assert.Equal(t, labelValue(pol.Name), updatedVAP.Labels[autogenSourceLabel])

	updatedBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.TODO(), bindingName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, vapName, updatedBinding.Spec.PolicyName)
	assert.Equal(t, labelValue(pol.Name), updatedBinding.Labels[autogenSourceLabel])
}

func TestReconcileAutogenVAP_Creation(t *testing.T) {
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.replicas > 0"},
			},
		},
	}
	vapName := "autogen-vpol-test-policy-defaults"
	bindingName := constructBindingName(vapName)

	client := fake.NewClientset()
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	c := &controller{
		client:           client,
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	autogenConfig := policiesv1beta1.ValidatingPolicyAutogen{
		Spec: &pol.Spec,
	}

	err := c.reconcileAutogenVAP(context.TODO(), pol, vapName, "defaults", autogenConfig, nil)
	assert.NoError(t, err)

	createdVAP, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.TODO(), vapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "object.spec.replicas > 0", createdVAP.Spec.Validations[0].Expression)

	createdBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.TODO(), bindingName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, vapName, createdBinding.Spec.PolicyName)
}

func TestReconcileAutogenVAP_Exceptions(t *testing.T) {
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
		},
	}
	vapName := "autogen-vpol-test-policy-cronjobs"

	client := fake.NewClientset()
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	c := &controller{
		client:           client,
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	autogenConfig := policiesv1beta1.ValidatingPolicyAutogen{
		Spec: &pol.Spec,
	}

	exceptions := []engineapi.GenericException{
		engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "test-ex"},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{Name: "ex1", Expression: "object.spec.template.spec.containers[0].name == 'foo'"},
				},
			},
		}),
	}

	err := c.reconcileAutogenVAP(context.TODO(), pol, vapName, "cronjobs", autogenConfig, exceptions)
	assert.NoError(t, err)

	createdVAP, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.TODO(), vapName, metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Len(t, createdVAP.Spec.MatchConditions, 1)
	assert.Contains(t, createdVAP.Spec.MatchConditions[0].Expression, "jobTemplate")
}
