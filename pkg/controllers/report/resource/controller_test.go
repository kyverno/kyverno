package resource

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

// matchResourcesFor selects the given core/v1 resource (e.g. "pods") on CREATE.
func matchResourcesFor(resource string) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{resource},
				},
			},
		}},
	}
}

// newTestController builds a resource-report controller backed by fake listers
// (seeded with the given policies) and a fake dclient, enough to exercise
// collectPolicyKinds without an API server.
func newTestController(t *testing.T, objects ...runtime.Object) *controller {
	t.Helper()
	kClient := versionedfake.NewSimpleClientset(objects...)
	factory := kyvernoinformer.NewSharedInformerFactory(kClient, 0)
	cpolInformer := factory.Kyverno().V1().ClusterPolicies()
	polInformer := factory.Kyverno().V1().Policies()
	vpolInformer := factory.Policies().V1beta1().ValidatingPolicies()
	// touch the informers so the factory actually starts them
	_ = cpolInformer.Informer()
	_ = polInformer.Informer()
	_ = vpolInformer.Informer()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	factory.Start(stop)
	factory.WaitForCacheSync(stop)
	return &controller{
		client:          dclient.NewEmptyFakeClient(),
		cpolLister:      cpolInformer.Lister(),
		polLister:       polInformer.Lister(),
		vpolLister:      vpolInformer.Lister(),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
	}
}

// A background-disabled ValidatingPolicy is never evaluated by the background
// scan controller, so the resource controller must not watch its resource kinds
// (watching + hashing them is the wasted work this guards against).
func TestCollectPolicyKinds_SkipsBackgroundDisabledValidatingPolicies(t *testing.T) {
	enabled := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-on-pods"},
		Spec:       policiesv1beta1.ValidatingPolicySpec{MatchConstraints: matchResourcesFor("pods")},
	}
	disabled := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-on-configmaps"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: matchResourcesFor("configmaps"),
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Background: &policiesv1beta1.BackgroundConfiguration{Enabled: ptr.To(false)},
			},
		},
	}
	c := newTestController(t, enabled, disabled)

	kinds, err := c.collectPolicyKinds()
	require.NoError(t, err)
	assert.True(t, kinds.Has("v1/Pod"), "kinds from a background-enabled policy must be collected, got %v", sets.List(kinds))
	assert.False(t, kinds.Has("v1/ConfigMap"), "kinds from a background-disabled policy must be skipped, got %v", sets.List(kinds))
}

// Same asymmetry on the traditional ClusterPolicy path: a policy with
// background:false should not contribute resource kinds to the watch set.
func TestCollectPolicyKinds_SkipsBackgroundDisabledClusterPolicies(t *testing.T) {
	validate := &kyvernov1.Validation{Message: "must comply"}
	enabled := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-on-pods"},
		Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
			Name:           "check",
			MatchResources: kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}}},
			Validation:     validate,
		}}},
	}
	disabled := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-on-configmaps"},
		Spec: kyvernov1.Spec{
			Background: ptr.To(false),
			Rules: []kyvernov1.Rule{{
				Name:           "check",
				MatchResources: kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}}},
				Validation:     validate,
			}},
		},
	}
	c := newTestController(t, enabled, disabled)

	kinds, err := c.collectPolicyKinds()
	require.NoError(t, err)
	assert.True(t, kinds.Has("Pod"), "kinds from a background-enabled ClusterPolicy must be collected, got %v", sets.List(kinds))
	assert.False(t, kinds.Has("ConfigMap"), "kinds from a background-disabled ClusterPolicy must be skipped, got %v", sets.List(kinds))
}
