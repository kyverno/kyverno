package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func warmUpTestScheme(t *testing.T) *kruntime.Scheme {
	t.Helper()
	scheme := kruntime.NewScheme()
	require.NoError(t, policiesv1beta1.Install(scheme))
	return scheme
}

// jsonPolicySpec is a ValidatingPolicy spec in JSON evaluation mode with the given validation, so it
// compiles without needing a fully-populated match constraint.
func jsonPolicySpec(expression string) policiesv1beta1.ValidatingPolicySpec {
	return policiesv1beta1.ValidatingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
			Mode: policieskyvernoio.EvaluationModeJSON,
		},
		Validations: []admissionregistrationv1.Validation{
			{Expression: expression},
		},
	}
}

func jsonPolicy(name string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       jsonPolicySpec("true"),
	}
}

func namespacedJSONPolicy(name, namespace string) *policiesv1beta1.NamespacedValidatingPolicy {
	return &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       jsonPolicySpec("true"),
	}
}

func fetchedNames(t *testing.T, rec *reconciler) []string {
	t.Helper()
	fetched, err := rec.Fetch(context.Background())
	require.NoError(t, err)
	names := make([]string, 0, len(fetched))
	for _, p := range fetched {
		names = append(names, p.Policy.GetName())
	}
	return names
}

// TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent is the regression test for the CEL engine
// startup enforcement gap: pre-existing ValidatingPolicy/NamespacedValidatingPolicy objects
// must be compiled and visible to Fetch() synchronously, without waiting on any workqueue-driven
// Reconcile call.
func TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent(t *testing.T) {
	cl := fake.NewClientBuilder().
		WithScheme(warmUpTestScheme(t)).
		WithObjects(jsonPolicy("cluster-policy"), namespacedJSONPolicy("ns-policy", "default")).
		Build()
	rec := newReconciler(compiler.NewCompiler(), cl, nil, false)

	require.Empty(t, fetchedNames(t, rec), "no policy should be visible before WarmUp or Reconcile ran")
	require.NoError(t, rec.WarmUp(context.Background()))
	assert.ElementsMatch(t, []string{"cluster-policy", "ns-policy"}, fetchedNames(t, rec))
}

// TestWarmUp_NoPolicies ensures WarmUp is a no-op (and doesn't error) when no policies exist yet.
func TestWarmUp_NoPolicies(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(warmUpTestScheme(t)).Build()
	rec := newReconciler(compiler.NewCompiler(), cl, nil, false)

	require.NoError(t, rec.WarmUp(context.Background()))
	assert.Empty(t, fetchedNames(t, rec))
}

// TestWarmUp_InvalidPolicyDoesNotBlockOthers ensures a policy that no longer compiles neither fails
// WarmUp (which would crash-loop the admission controller) nor prevents other policies from being
// warmed up.
func TestWarmUp_InvalidPolicyDoesNotBlockOthers(t *testing.T) {
	broken := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "a-broken-policy"},
		Spec:       jsonPolicySpec("this is not ( valid cel"),
	}
	cl := fake.NewClientBuilder().
		WithScheme(warmUpTestScheme(t)).
		WithObjects(broken, jsonPolicy("cluster-policy"), namespacedJSONPolicy("ns-policy", "default")).
		Build()
	rec := newReconciler(compiler.NewCompiler(), cl, nil, false)

	require.NoError(t, rec.WarmUp(context.Background()))
	assert.ElementsMatch(t, []string{"cluster-policy", "ns-policy"}, fetchedNames(t, rec))
}

// blockingCompiler wraps a compiler and lets a test pause the first Compile call.
type blockingCompiler struct {
	compiler.Compiler
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (c *blockingCompiler) Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*compiler.Policy, field.ErrorList) {
	c.once.Do(func() {
		close(c.started)
		<-c.release
	})
	return c.Compiler.Compile(policy, exceptions)
}

// TestReconcile_ConcurrentReconcileDoesNotStoreStaleVersion covers the race between WarmUp and the
// controller workqueue: WarmUp reads version 1 of a policy and is still compiling it when the
// policy is updated and the workqueue reconciles version 2. The result of the older reconcile must
// not overwrite the newer one.
func TestReconcile_ConcurrentReconcileDoesNotStoreStaleVersion(t *testing.T) {
	ctx := context.Background()
	policy := jsonPolicy("policy")
	policy.Labels = map[string]string{"version": "1"}
	cl := fake.NewClientBuilder().WithScheme(warmUpTestScheme(t)).WithObjects(policy).Build()
	blocking := &blockingCompiler{
		Compiler: compiler.NewCompiler(),
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	rec := newReconciler(blocking, cl, nil, false)
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(policy)}

	// the "warm-up" reconcile reads version 1 and blocks while compiling it
	stale := make(chan error, 1)
	go func() {
		_, err := rec.Reconcile(ctx, req)
		stale <- err
	}()
	<-blocking.started

	// the policy is updated and the "workqueue" reconciles version 2
	var current policiesv1beta1.ValidatingPolicy
	require.NoError(t, cl.Get(ctx, req.NamespacedName, &current))
	current.Labels["version"] = "2"
	require.NoError(t, cl.Update(ctx, &current))
	fresh := make(chan error, 1)
	go func() {
		_, err := rec.Reconcile(ctx, req)
		fresh <- err
	}()

	// Without serialization the fresh reconcile completes here and the stale one then overwrites
	// it. With serialization it cannot complete before the stale one is released.
	select {
	case err := <-fresh:
		require.NoError(t, err)
		fresh <- nil
	case <-time.After(200 * time.Millisecond):
	}
	close(blocking.release)
	require.NoError(t, <-stale)
	require.NoError(t, <-fresh)

	fetched, err := rec.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, fetched, 1)
	assert.Equal(t, "2", fetched[0].Policy.GetLabels()["version"])
}
