package admissionpolicygenerator

import (
	"context"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
)

func TestAutogenVAPName(t *testing.T) {
	assert.Equal(t, "vpol-check-pods-autogen-defaults", autogenVAPName("check-pods", "defaults"))
	assert.Equal(t, "vpol-check-pods-autogen-cronjobs", autogenVAPName("check-pods", "cronjobs"))
	assert.Equal(t, "vpol-check-pods-autogen-my-key", autogenVAPName("check-pods", "My_Key"))

	long := strings.Repeat("a", 250)
	name := autogenVAPName(long, "defaults")
	assert.LessOrEqual(t, len(constructBindingName(name)), validation.DNS1123SubdomainMaxLength)
	assert.Empty(t, validation.IsDNS1123Subdomain(name))
	assert.Empty(t, validation.IsDNS1123Subdomain(constructBindingName(name)))
	assert.Equal(t, name, autogenVAPName(long, "defaults"), "name must be stable")
	assert.NotEqual(t, name, autogenVAPName(long, "cronjobs"), "different keys must not collide")
}

func TestPruneStaleAutogenVAPs(t *testing.T) {
	pol := &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "check-pods"}}
	owned := map[string]string{autogenSourceLabel: pol.Name}
	vap := func(name string, lbls map[string]string) *admissionregistrationv1.ValidatingAdmissionPolicy {
		return &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls}}
	}
	binding := func(name, policyName string, lbls map[string]string) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
		return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls},
			Spec:       admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: policyName},
		}
	}
	active := autogenVAPName(pol.Name, "defaults")
	stale := autogenVAPName(pol.Name, "cronjobs")
	objects := []any{
		vap(active, owned),
		binding(constructBindingName(active), active, owned),
		vap(stale, owned),
		binding(constructBindingName(stale), stale, owned),
		// orphaned binding whose VAP is already gone; its name does not follow the convention
		binding("orphan", "vpol-check-pods-autogen-old", owned),
		// resources owned by another policy must never be touched
		vap("vpol-other-autogen-defaults", map[string]string{autogenSourceLabel: "other"}),
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
		assert.ElementsMatch(t, []string{active, "vpol-other-autogen-defaults"}, vapNames)
		assert.ElementsMatch(t, []string{constructBindingName(active)}, bindingNames)
	})

	t.Run("nil active set removes everything owned by the policy", func(t *testing.T) {
		c, client := newController()
		assert.NoError(t, c.deleteAutogenVAPs(context.TODO(), pol))
		vapNames, bindingNames := names(client)
		assert.ElementsMatch(t, []string{"vpol-other-autogen-defaults"}, vapNames)
		assert.Empty(t, bindingNames)
	})
}
