package admissionpolicygenerator

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	clocktesting "k8s.io/utils/clock/testing"
)

type denyResourceAuthChecker struct{ resource string }

func (c denyResourceAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: resource != c.resource}, nil
}

// delayRecorder records AddAfter without starting timers.
type delayRecorder struct {
	workqueue.TypedRateLimitingInterface[any]
	delays map[any]time.Duration
}

func (r *delayRecorder) AddAfter(item any, delay time.Duration) {
	r.delays[item] = delay
}

func expiringException(name, policyName, policyKind string, expiry time.Time) *policiesv1beta1.PolicyException {
	return &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: policyName, Kind: policyKind}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: name, Expression: "true"}},
			ExpiresAt:       &metav1.Time{Time: expiry},
		},
	}
}

// TestExpiryArmedBeforePermissionChecks covers the permission checks, which return without
// retrying, and policies that generate nothing, which must not be woken at expiry.
func TestExpiryArmedBeforePermissionChecks(t *testing.T) {
	type testCase struct{ kind, version, deny, ineligible string }
	var tests []testCase
	for _, deny := range []string{"validatingadmissionpolicies", "validatingadmissionpolicybindings"} {
		tests = append(tests, testCase{kind: "ValidatingPolicy", deny: deny})
	}
	for _, version := range []string{"v1", "v1beta1", "v1alpha1"} {
		for _, deny := range []string{"mutatingadmissionpolicies", "mutatingadmissionpolicybindings"} {
			tests = append(tests, testCase{kind: "MutatingPolicy", version: version, deny: deny})
		}
	}
	for _, ineligible := range []string{"disabled", "autogen"} {
		tests = append(tests, testCase{kind: "ValidatingPolicy", deny: "validatingadmissionpolicies", ineligible: ineligible})
	}
	for _, ineligible := range []string{"disabled", "autogen", "server side apply"} {
		tests = append(tests, testCase{kind: "MutatingPolicy", version: "v1", deny: "mutatingadmissionpolicies", ineligible: ineligible})
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.version+"/"+tt.deny+"/"+tt.ineligible, func(t *testing.T) {
			vpol := disallowPrivilegeEscalationVpol(nil)
			vpol.Kind = "ValidatingPolicy"
			if tt.ineligible != "autogen" {
				vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			}
			if tt.ineligible == "disabled" {
				*vpol.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = false
			}
			c, kubeClient, _ := newVAPTestController(t, vpol)
			generate := func() error { return c.handleVAPGeneration(context.Background(), engineapi.NewValidatingPolicy(vpol)) }
			name := vpol.Name
			if tt.kind == "MutatingPolicy" {
				mpol := &policiesv1beta1.MutatingPolicy{
					TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy"},
					ObjectMeta: metav1.ObjectMeta{Name: "policy"},
					Spec:       policiesv1beta1.MutatingPolicySpec{AutogenConfiguration: mapGenEnabled()},
				}
				switch tt.ineligible {
				case "disabled":
					*mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = false
				case "autogen":
					mpol.Spec.AutogenConfiguration = mapGenEnabledWithPodControllerAutogen()
					mpol.Spec.MatchConstraints = podsMatchConstraints()
				case "server side apply":
					mpol.Spec.EvaluationConfiguration = &policiesv1beta1.MutatingPolicyEvaluationConfiguration{UseServerSideApply: true}
				}
				switch tt.version {
				case "v1":
					indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
					c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(indexer)
					c.mapbindingV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(indexer)
				case "v1beta1":
					c.mapBetaLister, c.mapbindingBetaLister = &mockMAPBetaLister{}, &mockMAPBindingBetaLister{}
				case "v1alpha1":
					c.mapAlphaLister, c.mapbindingAlphaLister = &mockMAPAlphaLister{}, &mockMAPBindingAlphaLister{}
				}
				c.kyvernoClient = versionedfake.NewSimpleClientset(mpol)
				generate = func() error { return c.handleMAPGeneration(context.Background(), mpol) }
				name = mpol.Name
			}
			queue := &delayRecorder{delays: map[any]time.Duration{}}
			c.queue = queue
			c.checker = denyResourceAuthChecker{resource: tt.deny}
			c.polexEnabled, c.exceptionNamespace = true, "*"
			c.celpolexLister = newCELPolexLister(t, expiringException("exception", name, tt.kind, time.Now().Add(time.Hour)))

			require.NoError(t, generate())
			assert.Empty(t, kubeClient.Actions(), "permission denial must prevent native API writes")
			if tt.ineligible != "" {
				assert.Empty(t, queue.delays, "ineligible policies must not arm an expiry timer")
				return
			}
			require.Contains(t, queue.delays, tt.kind+"/"+name, "expiry must be armed despite permission denial")
			assert.Positive(t, queue.delays[tt.kind+"/"+name])
		})
	}
}

// TestGenerationRearmsExceptionExpiry walks a policy through two exception deadlines and an
// extension, checking the armed delay and the generated VAP's match conditions at each step.
func TestGenerationRearmsExceptionExpiry(t *testing.T) {
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Kind = "ValidatingPolicy"
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	c, kubeClient, _ := newVAPTestController(t, vpol)
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer)
	c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer)
	exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.celpolexLister = policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer)
	c.polexEnabled, c.exceptionNamespace = true, "*"
	queue := &delayRecorder{delays: map[any]time.Duration{}}
	c.queue = queue
	key := vpol.Kind + "/" + vpol.Name

	generate := func(wantExpiry time.Time, wantConditions int) {
		t.Helper()
		clear(queue.delays)
		before := time.Now()
		require.NoError(t, c.handleVAPGeneration(context.Background(), engineapi.NewValidatingPolicy(vpol)))
		after := time.Now()
		if wantExpiry.IsZero() {
			assert.Empty(t, queue.delays)
		} else {
			require.Contains(t, queue.delays, key)
			assert.GreaterOrEqual(t, queue.delays[key], wantExpiry.Sub(after))
			assert.LessOrEqual(t, queue.delays[key], wantExpiry.Sub(before))
		}
		// Feed the generated objects back to the listers so the next generation updates them.
		vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), "vpol-"+vpol.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, vap.Spec.MatchConditions, wantConditions)
		vap.ResourceVersion = "1"
		require.NoError(t, vapIndexer.Update(vap))
		binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), "vpol-"+vpol.Name+"-binding", metav1.GetOptions{})
		require.NoError(t, err)
		binding.ResourceVersion = "1"
		require.NoError(t, bindingIndexer.Update(binding))
	}
	setExpiry := func(exception *policiesv1beta1.PolicyException, expiry time.Time) {
		exception.Spec.ExpiresAt = &metav1.Time{Time: expiry}
		require.NoError(t, exceptionIndexer.Update(exception))
	}

	now := time.Now()
	first := expiringException("first", vpol.Name, vpol.Kind, now.Add(time.Hour))
	second := expiringException("second", vpol.Name, vpol.Kind, now.Add(2*time.Hour))
	require.NoError(t, exceptionIndexer.Add(first))
	require.NoError(t, exceptionIndexer.Add(second))
	generate(now.Add(time.Hour), 3)
	setExpiry(first, now.Add(-time.Hour))
	generate(now.Add(2*time.Hour), 2)
	setExpiry(second, now.Add(3*time.Hour))
	generate(now.Add(3*time.Hour), 2)
	setExpiry(second, now.Add(-time.Hour))
	generate(time.Time{}, 1)

	t.Run("expired during generation", func(t *testing.T) {
		// An exception listed as live can expire before it is armed; it must still requeue now.
		clear(queue.delays)
		c.requeueAtExpiry(vpol.Name, vpol.Kind, []policiesv1beta1.PolicyException{*expiringException("late", vpol.Name, vpol.Kind, time.Now().Add(-time.Second))})
		require.Contains(t, queue.delays, key)
		assert.LessOrEqual(t, queue.delays[key], time.Duration(0))
	})
	t.Run("nil expiry", func(t *testing.T) {
		never := *expiringException("never", vpol.Name, vpol.Kind, time.Time{})
		never.Spec.ExpiresAt = nil
		clear(queue.delays)
		c.requeueAtExpiry(vpol.Name, vpol.Kind, []policiesv1beta1.PolicyException{never})
		assert.Empty(t, queue.delays, "an exception without expiresAt arms nothing")
		expiry := time.Now().Add(time.Hour)
		before := time.Now()
		c.requeueAtExpiry(vpol.Name, vpol.Kind, []policiesv1beta1.PolicyException{never, *expiringException("dated", vpol.Name, vpol.Kind, expiry)})
		after := time.Now()
		require.Contains(t, queue.delays, key)
		assert.GreaterOrEqual(t, queue.delays[key], expiry.Sub(after))
		assert.LessOrEqual(t, queue.delays[key], expiry.Sub(before))
	})
}

// TestRequeueAtExpiryDeliversPolicyKeyToWorker checks that the delayed key reaches the
// controller's worker once the deadline passes.
func TestRequeueAtExpiryDeliversPolicyKeyToWorker(t *testing.T) {
	now := time.Now()
	fakeClock := clocktesting.NewFakeClock(now)
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Clock: fakeClock},
	)
	c := &controller{queue: queue}
	c.requeueAtExpiry("vpol", "ValidatingPolicy", []policiesv1beta1.PolicyException{*expiringException("exception", "vpol", "ValidatingPolicy", now.Add(time.Hour))})
	assert.Zero(t, queue.Len(), "the deadline is not ready yet")

	ctx, cancel := context.WithCancel(context.Background())
	keys := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		controllerutils.Run(ctx, logr.Discard(), "expiry-test", time.Second, queue, 1, 0,
			func(_ context.Context, _ logr.Logger, key, _, _ string) error {
				keys <- key
				return nil
			})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	fakeClock.Step(2 * time.Hour)
	select {
	case key := <-keys:
		assert.Equal(t, "ValidatingPolicy/vpol", key)
	case <-time.After(10 * time.Second):
		t.Fatal("the worker did not receive the policy key after the fake-clock deadline")
	}
}
