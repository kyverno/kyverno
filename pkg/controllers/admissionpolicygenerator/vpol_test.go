package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

func newTestQueue(t *testing.T) workqueue.TypedRateLimitingInterface[any] {
	t.Helper()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	t.Cleanup(queue.ShutDown)
	return queue
}

// drainQueue returns every item currently in the queue.
func drainQueue(queue workqueue.TypedRateLimitingInterface[any]) []any {
	var items []any
	for queue.Len() > 0 {
		item, _ := queue.Get()
		queue.Done(item)
		items = append(items, item)
	}
	return items
}

func newNvpol(namespace, name, message string) *policiesv1beta1.NamespacedValidatingPolicy {
	return &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				ValidatingAdmissionPolicy: &policiesv1beta1.VapGenerationConfiguration{
					Enabled: ptr.To(true),
				},
			},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "true",
				Message:    message,
			}},
		},
	}
}

// newNvpolTestController builds a controller backed by fake clientsets, with the given
// namespaced policies in the lister and the given native objects in the kube clientset.
func newNvpolTestController(t *testing.T, nvpols []*policiesv1beta1.NamespacedValidatingPolicy, kubeObjects ...runtime.Object) (*controller, *kubefake.Clientset) {
	t.Helper()
	kubeClient := kubefake.NewSimpleClientset(kubeObjects...)
	nvpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	var kyvernoObjects []runtime.Object
	for _, nvpol := range nvpols {
		require.NoError(t, nvpolIndexer.Add(nvpol))
		kyvernoObjects = append(kyvernoObjects, nvpol)
	}
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	vapBindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range kubeObjects {
		switch obj.(type) {
		case *admissionregistrationv1.ValidatingAdmissionPolicy:
			require.NoError(t, vapIndexer.Add(obj))
		case *admissionregistrationv1.ValidatingAdmissionPolicyBinding:
			require.NoError(t, vapBindingIndexer.Add(obj))
		}
	}
	c := &controller{
		client:           kubeClient,
		kyvernoClient:    versionedfake.NewSimpleClientset(kyvernoObjects...),
		discoveryClient:  dclient.NewEmptyFakeClient().Discovery(),
		eventGen:         event.NewFake(),
		checker:          permissiveAuthChecker{},
		nvpolLister:      policiesv1beta1listers.NewNamespacedValidatingPolicyLister(nvpolIndexer),
		celpolexLister:   policiesv1beta1listers.NewPolicyExceptionLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(vapBindingIndexer),
		queue:            newTestQueue(t),
	}
	return c, kubeClient
}

func TestEnqueueVP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  policiesv1beta1.ValidatingPolicyLike
		want any
	}{{
		name: "cluster-scoped policy uses a namespace/name key",
		obj:  &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"}},
		want: "ValidatingPolicy/test-vpol",
	}, {
		name: "namespaced policy uses an explicit key",
		obj:  &policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-nvpol", Namespace: "test-namespace"}},
		want: cache.ExplicitKey("NamespacedValidatingPolicy/test-namespace/test-nvpol"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &controller{queue: newTestQueue(t)}
			c.enqueueVP(tt.obj)
			assert.Equal(t, []any{tt.want}, drainQueue(c.queue))
		})
	}
}

func TestHandlersVP_NamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	key := cache.ExplicitKey("NamespacedValidatingPolicy/default/test-nvpol")
	oldPol := newNvpol("default", "test-nvpol", "msg1")
	newPol := newNvpol("default", "test-nvpol", "msg2")
	tests := []struct {
		name   string
		handle func(c *controller)
		want   []any
	}{{
		name:   "add",
		handle: func(c *controller) { c.addVP(oldPol) },
		want:   []any{key},
	}, {
		name:   "update with a changed spec",
		handle: func(c *controller) { c.updateVP(oldPol, newPol) },
		want:   []any{key},
	}, {
		name:   "update with an unchanged spec",
		handle: func(c *controller) { c.updateVP(oldPol, oldPol.DeepCopy()) },
	}, {
		name:   "delete",
		handle: func(c *controller) { c.deleteVP(newPol) },
		want:   []any{key},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &controller{queue: newTestQueue(t)}
			tt.handle(c)
			assert.Equal(t, tt.want, drainQueue(c.queue))
		})
	}
}

func TestParseNamespacedPolicyKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key           string
		wantNamespace string
		wantName      string
		wantOK        bool
	}{
		{key: "NamespacedValidatingPolicy/team-a/foo", wantNamespace: "team-a", wantName: "foo", wantOK: true},
		{key: "NamespacedValidatingPolicy/team-a"},
		{key: "NamespacedValidatingPolicy//foo"},
		{key: "NamespacedValidatingPolicy/team-a/"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			namespace, name, ok := parseNamespacedPolicyKey(tt.key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantNamespace, namespace)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestReconcile_NamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	nvpol := newNvpol("team-a", "foo", "denied")
	c, kubeClient := newNvpolTestController(t, []*policiesv1beta1.NamespacedValidatingPolicy{nvpol})

	require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

	vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "nvpol-team-a.foo", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, vap.OwnerReferences, "a cluster-scoped VAP cannot be owned by a namespaced policy")
	assert.Equal(t, "team-a", vap.Annotations[admissionpolicy.AnnotationSourcePolicyNamespace])
	assert.Equal(t, "foo", vap.Annotations[admissionpolicy.AnnotationSourcePolicyName])
	require.NotNil(t, vap.Spec.MatchConstraints.NamespaceSelector)
	assert.Equal(t, []metav1.LabelSelectorRequirement{{
		Key:      "kubernetes.io/metadata.name",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"team-a"},
	}}, vap.Spec.MatchConstraints.NamespaceSelector.MatchExpressions)

	binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "nvpol-team-a.foo-binding", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "nvpol-team-a.foo", binding.Spec.PolicyName)
	assert.Empty(t, binding.OwnerReferences)
}

func TestReconcile_NamespacedValidatingPolicyKeepsNamespaceSelector(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	nvpol := newNvpol("team-a", "foo", "denied")
	nvpol.Spec.MatchConstraints.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	c, kubeClient := newNvpolTestController(t, []*policiesv1beta1.NamespacedValidatingPolicy{nvpol})

	require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

	vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "nvpol-team-a.foo", metav1.GetOptions{})
	require.NoError(t, err)
	selector := vap.Spec.MatchConstraints.NamespaceSelector
	require.NotNil(t, selector)
	assert.Equal(t, map[string]string{"env": "prod"}, selector.MatchLabels)
	assert.Equal(t, []metav1.LabelSelectorRequirement{{
		Key:      "kubernetes.io/metadata.name",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"team-a"},
	}}, selector.MatchExpressions)
	// the source policy is left untouched
	assert.Empty(t, nvpol.Spec.MatchConstraints.NamespaceSelector.MatchExpressions)
}

func TestReconcile_NamespacedValidatingPolicyOnlyMatchesNamespacedResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	nvpol := newNvpol("team-a", "foo", "denied")
	nvpol.Spec.MatchConstraints.ResourceRules = append(nvpol.Spec.MatchConstraints.ResourceRules, admissionregistrationv1.NamedRuleWithOperations{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"nodes"},
				Scope:       ptr.To(admissionregistrationv1.ClusterScope),
			},
		},
	})
	c, kubeClient := newNvpolTestController(t, []*policiesv1beta1.NamespacedValidatingPolicy{nvpol})

	require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

	vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "nvpol-team-a.foo", metav1.GetOptions{})
	require.NoError(t, err)
	rules := vap.Spec.MatchConstraints.ResourceRules
	require.Len(t, rules, 1, "the cluster-scoped rule must be dropped")
	assert.Equal(t, []string{"configmaps"}, rules[0].Resources)
	require.NotNil(t, rules[0].Scope)
	assert.Equal(t, admissionregistrationv1.NamespacedScope, *rules[0].Scope)
	// the source policy is left untouched
	assert.Nil(t, nvpol.Spec.MatchConstraints.ResourceRules[0].Scope)
}

func TestReconcile_NamespacedValidatingPolicyNotGenerated(t *testing.T) {
	t.Parallel()
	clusterScoped := func(p *policiesv1beta1.NamespacedValidatingPolicy) {
		p.Spec.MatchConstraints.ResourceRules[0].Scope = ptr.To(admissionregistrationv1.ClusterScope)
	}
	tests := []struct {
		name     string
		mutate   func(*policiesv1beta1.NamespacedValidatingPolicy)
		existing bool
	}{{
		name:   "only cluster-scoped rules",
		mutate: clusterScoped,
	}, {
		name:   "no match constraints",
		mutate: func(p *policiesv1beta1.NamespacedValidatingPolicy) { p.Spec.MatchConstraints = nil },
	}, {
		name:     "changed to only cluster-scoped rules after a VAP was generated",
		mutate:   clusterScoped,
		existing: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			nvpol := newNvpol("team-a", "foo", "denied")
			tt.mutate(nvpol)
			var existing []runtime.Object
			if tt.existing {
				existing = append(existing,
					&admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo"}},
					&admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo-binding"}},
				)
			}
			c, kubeClient := newNvpolTestController(t, []*policiesv1beta1.NamespacedValidatingPolicy{nvpol}, existing...)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

			_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "nvpol-team-a.foo", metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "no VAP expected: got err=%v", err)
			_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "nvpol-team-a.foo-binding", metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "no binding expected: got err=%v", err)
			updated, err := c.kyvernoClient.PoliciesV1beta1().NamespacedValidatingPolicies("team-a").Get(ctx, "foo", metav1.GetOptions{})
			require.NoError(t, err)
			assert.False(t, updated.Status.Generated)
		})
	}
}

func TestReconcile_DeletedNamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo"}}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo-binding"}}
	c, kubeClient := newNvpolTestController(t, nil, vap, binding)

	require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vap.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "the generated VAP must be deleted: got err=%v", err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "the generated binding must be deleted: got err=%v", err)

	// nothing left to delete is not an error
	require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))
}

// vapGenerationOff turns off ValidatingAdmissionPolicy generation and keeps the other defaults.
type vapGenerationOff struct {
	toggle.Toggles
}

func (vapGenerationOff) GenerateValidatingAdmissionPolicy() bool { return false }

func TestReconcile_NamespacedValidatingPolicyGenerationDisabled(t *testing.T) {
	t.Parallel()
	ctx := toggle.NewContext(context.Background(), vapGenerationOff{toggle.FromContext(context.Background())})
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo"}}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo-binding"}}
	generated := newNvpol("team-a", "foo", "denied")
	generated.Status.Generated = true
	tests := []struct {
		name   string
		nvpols []*policiesv1beta1.NamespacedValidatingPolicy
	}{
		{name: "policy still exists", nvpols: []*policiesv1beta1.NamespacedValidatingPolicy{generated}},
		{name: "policy deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, kubeClient := newNvpolTestController(t, tt.nvpols, vap.DeepCopy(), binding.DeepCopy())

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "NamespacedValidatingPolicy/team-a/foo", "", ""))

			_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vap.Name, metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "the generated VAP must be deleted: got err=%v", err)
			_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "the generated binding must be deleted: got err=%v", err)
			if len(tt.nvpols) > 0 {
				updated, err := c.kyvernoClient.PoliciesV1beta1().NamespacedValidatingPolicies("team-a").Get(ctx, "foo", metav1.GetOptions{})
				require.NoError(t, err)
				assert.False(t, updated.Status.Generated, "the policy must no longer report a generated VAP")
			}
		})
	}
}

// A generated VAP whose policy was deleted while the controller was down is listed again on startup,
// requeues its policy and is removed.
func TestOrphanedGeneratedVAPIsRemoved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	annotations := map[string]string{
		admissionpolicy.AnnotationSourcePolicyNamespace: "team-a",
		admissionpolicy.AnnotationSourcePolicyName:      "foo",
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo", Annotations: annotations}}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "nvpol-team-a.foo-binding", Annotations: annotations}}
	c, kubeClient := newNvpolTestController(t, nil, vap, binding)

	c.addVAP(vap)
	items := drainQueue(c.queue)
	require.Len(t, items, 1)
	key, ok := items[0].(cache.ExplicitKey)
	require.True(t, ok)
	require.NoError(t, c.reconcile(ctx, logr.Discard(), string(key), "", ""))

	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vap.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "the orphaned VAP must be deleted: got err=%v", err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "the orphaned binding must be deleted: got err=%v", err)
}

func TestEnqueueCELException_NamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	c, _ := newNvpolTestController(t, []*policiesv1beta1.NamespacedValidatingPolicy{
		newNvpol("team-a", "foo", ""),
		newNvpol("team-b", "foo", ""),
		newNvpol("team-a", "bar", ""),
	})
	c.enqueueCELException(&policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "foo", Kind: "NamespacedValidatingPolicy"}},
		},
	})
	assert.ElementsMatch(t, []any{
		cache.ExplicitKey("NamespacedValidatingPolicy/team-a/foo"),
		cache.ExplicitKey("NamespacedValidatingPolicy/team-b/foo"),
	}, drainQueue(c.queue))
}

func TestEnqueueVAP_SourceNamespacedPolicy(t *testing.T) {
	t.Parallel()
	annotations := map[string]string{
		admissionpolicy.AnnotationSourcePolicyNamespace: "team-a",
		admissionpolicy.AnnotationSourcePolicyName:      "foo",
	}
	key := cache.ExplicitKey("NamespacedValidatingPolicy/team-a/foo")

	c := &controller{queue: newTestQueue(t)}
	c.enqueueVAP(&admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}})
	c.enqueueVAPbinding(&admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}})
	// the workqueue deduplicates identical keys
	assert.Equal(t, []any{key}, drainQueue(c.queue))

	// objects without the annotations and without owners are ignored
	c.enqueueVAP(&admissionregistrationv1.ValidatingAdmissionPolicy{})
	assert.Empty(t, drainQueue(c.queue))
}
