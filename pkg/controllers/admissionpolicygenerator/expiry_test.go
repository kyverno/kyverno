package admissionpolicygenerator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
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

// TestExpiryArmedBeforePermissionChecks covers permission check failures, which return an error
// to retain a retry, and policies that generate nothing, which must not be woken at expiry.
func TestExpiryArmedBeforePermissionChecks(t *testing.T) {
	type testCase struct{ kind, version, deny, ineligible string; conflict bool }
	var tests []testCase
	for _, deny := range []string{"validatingadmissionpolicies", "validatingadmissionpolicybindings"} {
		tests = append(tests, testCase{kind: "ValidatingPolicy", deny: deny})
		tests = append(tests, testCase{kind: "ValidatingPolicy", deny: deny, conflict: true})
	}
	for _, version := range []string{"v1", "v1beta1", "v1alpha1"} {
		for _, deny := range []string{"mutatingadmissionpolicies", "mutatingadmissionpolicybindings"} {
			tests = append(tests, testCase{kind: "MutatingPolicy", version: version, deny: deny})
			tests = append(tests, testCase{kind: "MutatingPolicy", version: version, deny: deny, conflict: true})
		}
	}
	for _, ineligible := range []string{"disabled", "autogen"} {
		tests = append(tests, testCase{kind: "ValidatingPolicy", deny: "validatingadmissionpolicies", ineligible: ineligible})
	}
	for _, ineligible := range []string{"disabled", "autogen", "server side apply"} {
		tests = append(tests, testCase{kind: "MutatingPolicy", version: "v1", deny: "mutatingadmissionpolicies", ineligible: ineligible})
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("%s/%s/%s/%s/conflict=%t", tt.kind, tt.version, tt.deny, tt.ineligible, tt.conflict)
		t.Run(testName, func(t *testing.T) {
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
				name = mpol.Name
				c.kyvernoClient = versionedfake.NewSimpleClientset(mpol)
				version :=  admissionpolicy.MutatingAdmissionPolicyVersion(tt.version)
				generate = func() error {
					return c.handleMAPGenerationWithVersion(context.Background(), mpol, version)
				}
			}
			if tt.conflict {
				fakeKyverno := versionedfake.NewSimpleClientset()
				conflictErr := apierrors.NewConflict(schema.GroupResource{Group: "policies.kyverno.io", Resource: strings.ToLower(tt.kind) + "s"}, name, errors.New("conflict"))
				fakeKyverno.PrependReactor("update", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() == "status" {
						return true, nil, conflictErr
					}
					return false, nil, nil
				})
				c.kyvernoClient = fakeKyverno
			}
			c.checker = denyResourceAuthChecker{resource: tt.deny}
			queue := &delayRecorder{delays: map[any]time.Duration{}}
			c.queue = queue
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			c.celpolexLister = policiesv1beta1listers.NewPolicyExceptionLister(indexer)
			c.polexEnabled, c.exceptionNamespace = true, "*"
			deadline := time.Now().Add(time.Hour)
			require.NoError(t, indexer.Add(expiringException("exception", name, tt.kind, deadline)))

			key := tt.kind + "/" + name
			if tt.ineligible != "" {
				require.NoError(t, generate())
				assert.NotContains(t, queue.delays, key, "ineligible policies must not schedule expiry requeues")
			} else {
				err := generate()
				require.Error(t, err)
				if tt.conflict {
					assert.True(t, apierrors.IsConflict(err), "expected resource-version conflict error to be preserved, got %v", err)
				} else {
					assert.Contains(t, err.Error(), "insufficient permissions")
				}
				assert.Contains(t, queue.delays, key, "expiry must be armed even when permission checks fail")
			}
			// Confirm the permission check actually fired: nothing was created.
			if tt.kind == "MutatingPolicy" {
				switch tt.version {
				case "v1":
					maps, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, maps.Items)
					bindings, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, bindings.Items)
				case "v1beta1":
					maps, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, maps.Items)
					bindings, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, bindings.Items)
				case "v1alpha1":
					maps, err := kubeClient.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, maps.Items)
					bindings, err := kubeClient.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{})
					require.NoError(t, err)
					assert.Empty(t, bindings.Items)
				}
			} else {
				vaps, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				assert.Empty(t, vaps.Items)
				bindings, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{})
				require.NoError(t, err)
				assert.Empty(t, bindings.Items)
			}
		})
	}
}

// TestGenerationRearmsExceptionExpiry runs several generations of the same policy as its
// exceptions expire and are pushed back, checking that the requeue timer tracks the moving
// earliest deadline and that the generated ValidatingAdmissionPolicy's match conditions update.
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

	generate := func(wantExpiry time.Time, wantConditions int, expectedRemainingExceptionName string) {
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
		if expectedRemainingExceptionName != "" {
			found := false
			for _, mc := range vap.Spec.MatchConditions {
				if mc.Name == expectedRemainingExceptionName {
					found = true
					break
				}
			}
			assert.True(t, found, "expected matchCondition %s to remain on VAP", expectedRemainingExceptionName)
		}
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
	require.NoError(t, exceptionIndexer.Add(expiringException("other-policy", "other", "ValidatingPolicy", now.Add(30*time.Minute))))
	generate(now.Add(time.Hour), 3, "first")
	setExpiry(first, now.Add(-time.Hour))
	generate(now.Add(2*time.Hour), 2, "second")
	setExpiry(second, now.Add(3*time.Hour))
	generate(now.Add(3*time.Hour), 2, "second")
	setExpiry(second, now.Add(-time.Hour))
	generate(time.Time{}, 1, "")

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

// TestMAPGenerationRearmsExceptionExpiry tests that MutatingAdmissionPolicy generation
// arms requeue at the earliest PolicyException expiry across v1, v1beta1, and v1alpha1,
// and regenerates the MAP removing the exception matchConditions once expired.
func TestMAPGenerationRearmsExceptionExpiry(t *testing.T) {
	versions := []admissionpolicy.MutatingAdmissionPolicyVersion{
		admissionpolicy.MutatingAdmissionPolicyVersionV1,
		admissionpolicy.MutatingAdmissionPolicyVersionV1beta1,
		admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1,
	}

	for _, ver := range versions {
		t.Run(string(ver), func(t *testing.T) {
			mpol := &policiesv1beta1.MutatingPolicy{
				TypeMeta: metav1.TypeMeta{Kind: "MutatingPolicy"},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-map-rearm",
					UID:  "test-uid",
				},
				Spec: policiesv1beta1.MutatingPolicySpec{
					AutogenConfiguration: mapGenEnabled(),
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"},
										Resources:   []string{"configmaps"},
									},
								},
							},
						},
					},
					Mutations: []admissionregistrationv1alpha1.Mutation{
						{
							PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
							ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
								Expression: `Object{spec: Object.spec{containers: [Object.spec.containers[0]{name: "injected"}]}}`,
							},
						},
					},
				},
			}

			kubeClient := kubefake.NewSimpleClientset()
			kyvernoClient := versionedfake.NewSimpleClientset(mpol)
			queue := &delayRecorder{delays: map[any]time.Duration{}}
			exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

			c := &controller{
				client:             kubeClient,
				kyvernoClient:      kyvernoClient,
				discoveryClient:    dclient.NewEmptyFakeClient().Discovery(),
				eventGen:           event.NewFake(),
				checker:            permissiveAuthChecker{},
				queue:              queue,
				celpolexLister:     policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer),
				polexEnabled:       true,
				exceptionNamespace: "*",
			}

			mapName := "mpol-" + mpol.Name
			bindingName := constructBindingName(mapName)
			key := "MutatingPolicy/" + mpol.Name

			var mapV1Indexer, mapbindingV1Indexer cache.Indexer
			var mapBetaIndexer, mapbindingBetaIndexer cache.Indexer
			var mapAlphaIndexer, mapbindingAlphaIndexer cache.Indexer

			switch ver {
			case admissionpolicy.MutatingAdmissionPolicyVersionV1:
				mapV1Indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				mapbindingV1Indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapV1Indexer)
				c.mapbindingV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(mapbindingV1Indexer)
			case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
				mapBetaIndexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				mapbindingBetaIndexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				c.mapBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyLister(mapBetaIndexer)
				c.mapbindingBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyBindingLister(mapbindingBetaIndexer)
			case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
				mapAlphaIndexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				mapbindingAlphaIndexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				c.mapAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyLister(mapAlphaIndexer)
				c.mapbindingAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyBindingLister(mapbindingAlphaIndexer)
			}

			generate := func(wantExpiry time.Time, wantConditions int, expectedRemainingExceptionName string) {
				t.Helper()
				clear(queue.delays)
				before := time.Now()
				require.NoError(t, c.handleMAPGenerationWithVersion(context.Background(), mpol, ver))
				after := time.Now()

				if wantExpiry.IsZero() {
					assert.Empty(t, queue.delays)
				} else {
					require.Contains(t, queue.delays, key)
					assert.GreaterOrEqual(t, queue.delays[key], wantExpiry.Sub(after))
					assert.LessOrEqual(t, queue.delays[key], wantExpiry.Sub(before))
				}

				// Verify generated match conditions and update indexers
				switch ver {
				case admissionpolicy.MutatingAdmissionPolicyVersionV1:
					obj, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
					require.NoError(t, err)
					assert.Len(t, obj.Spec.MatchConditions, wantConditions)
					if expectedRemainingExceptionName != "" {
						found := false
						for _, mc := range obj.Spec.MatchConditions {
							if mc.Name == expectedRemainingExceptionName {
								found = true
								break
							}
						}
						assert.True(t, found, "expected matchCondition %s to remain on MAP", expectedRemainingExceptionName)
					}
					obj.ResourceVersion = "1"
					require.NoError(t, mapV1Indexer.Update(obj))
					b, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
					require.NoError(t, err)
					b.ResourceVersion = "1"
					require.NoError(t, mapbindingV1Indexer.Update(b))
				case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
					obj, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
					require.NoError(t, err)
					assert.Len(t, obj.Spec.MatchConditions, wantConditions)
					if expectedRemainingExceptionName != "" {
						found := false
						for _, mc := range obj.Spec.MatchConditions {
							if mc.Name == expectedRemainingExceptionName {
								found = true
								break
							}
						}
						assert.True(t, found, "expected matchCondition %s to remain on MAP", expectedRemainingExceptionName)
					}
					obj.ResourceVersion = "1"
					require.NoError(t, mapBetaIndexer.Update(obj))
					b, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
					require.NoError(t, err)
					b.ResourceVersion = "1"
					require.NoError(t, mapbindingBetaIndexer.Update(b))
				case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
					obj, err := kubeClient.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
					require.NoError(t, err)
					assert.Len(t, obj.Spec.MatchConditions, wantConditions)
					if expectedRemainingExceptionName != "" {
						found := false
						for _, mc := range obj.Spec.MatchConditions {
							if mc.Name == expectedRemainingExceptionName {
								found = true
								break
							}
						}
						assert.True(t, found, "expected matchCondition %s to remain on MAP", expectedRemainingExceptionName)
					}
					obj.ResourceVersion = "1"
					require.NoError(t, mapAlphaIndexer.Update(obj))
					b, err := kubeClient.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
					require.NoError(t, err)
					b.ResourceVersion = "1"
					require.NoError(t, mapbindingAlphaIndexer.Update(b))
				}
			}

			setExpiry := func(exception *policiesv1beta1.PolicyException, expiry time.Time) {
				exception.Spec.ExpiresAt = &metav1.Time{Time: expiry}
				require.NoError(t, exceptionIndexer.Update(exception))
			}

			now := time.Now()
			first := expiringException("first", mpol.Name, "MutatingPolicy", now.Add(time.Hour))
			second := expiringException("second", mpol.Name, "MutatingPolicy", now.Add(2*time.Hour))
			require.NoError(t, exceptionIndexer.Add(first))
			require.NoError(t, exceptionIndexer.Add(second))
			require.NoError(t, exceptionIndexer.Add(expiringException("other-kind", mpol.Name, "ValidatingPolicy", now.Add(30*time.Minute))))
			require.NoError(t, exceptionIndexer.Add(expiringException("other-policy", "other", "MutatingPolicy", now.Add(30*time.Minute))))

			// Initial generation with two active exceptions: 2 matchConditions
			generate(now.Add(time.Hour), 2, "first")

			// First exception expires: requeue targets second, 1 matchCondition remains
			setExpiry(first, now.Add(-time.Hour))
			generate(now.Add(2*time.Hour), 1, "second")

			// Second exception expires: no remaining exceptions, 0 matchConditions remain
			setExpiry(second, now.Add(-time.Hour))
			generate(time.Time{}, 0, "")
		})
	}
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

type erroringPolexLister struct{}

func (erroringPolexLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, errors.New("polex list error")
}

func (erroringPolexLister) PolicyExceptions(namespace string) policiesv1beta1listers.PolicyExceptionNamespaceLister {
	return nil
}

// TestVAPGenerationIneligibleDueToValidations verifies that an exception with Validations
// skips native ValidatingAdmissionPolicy generation while still arming the expiry requeue.
func TestVAPGenerationIneligibleDueToValidations(t *testing.T) {
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Kind = "ValidatingPolicy"
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	c, kubeClient, _ := newVAPTestController(t, vpol)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.celpolexLister = policiesv1beta1listers.NewPolicyExceptionLister(indexer)
	c.polexEnabled, c.exceptionNamespace = true, "*"
	queue := &delayRecorder{delays: map[any]time.Duration{}}
	c.queue = queue

	invalidPolex := expiringException("invalid-exception", vpol.Name, vpol.Kind, time.Now().Add(time.Hour))
	invalidPolex.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "false"}}
	require.NoError(t, indexer.Add(invalidPolex))

	err := c.handleVAPGeneration(context.Background(), engineapi.NewValidatingPolicy(vpol))
	require.NoError(t, err)

	key := vpol.Kind + "/" + vpol.Name
	assert.Contains(t, queue.delays, key, "requeue must still be armed even when native generation is ineligible due to validations")

	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), "vpol-"+vpol.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), "vpol-"+vpol.Name+"-binding", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))

	updated, err := c.kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(context.Background(), vpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, updated.Status.Generated)
}

// TestGenerationCELExceptionsError verifies that error paths during exception retrieval
// return formatted errors for both VAP and MAP generation.
func TestGenerationCELExceptionsError(t *testing.T) {
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Kind = "ValidatingPolicy"
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	c, _, _ := newVAPTestController(t, vpol)
	c.polexEnabled, c.exceptionNamespace = true, "*"
	c.celpolexLister = erroringPolexLister{}

	err := c.handleVAPGeneration(context.Background(), engineapi.NewValidatingPolicy(vpol))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get celexceptions")

	mpol := &policiesv1beta1.MutatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration: mapGenEnabled(),
		},
	}
	c.kyvernoClient = versionedfake.NewSimpleClientset(mpol)
	err = c.handleMAPGenerationWithVersion(context.Background(), mpol, admissionpolicy.MutatingAdmissionPolicyVersionV1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get celexceptions")
}

type switchableAuthChecker struct {
	allowed bool
}

func (s *switchableAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: s.allowed}, nil
}

// TestExpiryPermissionFailureRecoveryRemovalOfCondition tests the lifecycle requested by maintainers:
// expiry -> permission failure -> recovery -> removal of the expired condition.
// When an exception expires and the timer fires, if permissions are down, reconciliation must
// return an error to retain a retry in the workqueue. Once permissions recover, the retried
// reconciliation removes the expired matchCondition.
func TestExpiryPermissionFailureRecoveryRemovalOfCondition(t *testing.T) {
	t.Run("ValidatingPolicy", func(t *testing.T) {
		vpol := disallowPrivilegeEscalationVpol(nil)
		vpol.Kind = "ValidatingPolicy"
		vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
		c, kubeClient, kyvernoClient := newVAPTestController(t, vpol)

		vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer)
		c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer)

		vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		require.NoError(t, vpolIndexer.Add(vpol))
		c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer)

		exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		c.celpolexLister = policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer)
		c.polexEnabled, c.exceptionNamespace = true, "*"

		auth := &switchableAuthChecker{allowed: true}
		c.checker = auth
		queue := &delayRecorder{delays: map[any]time.Duration{}}
		c.queue = queue

		// 1. Initial generation: active exception expiring in 1 hour
		now := time.Now()
		polex := expiringException("exp-1", vpol.Name, vpol.Kind, now.Add(time.Hour))
		require.NoError(t, exceptionIndexer.Add(polex))

		require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))

		vapName := "vpol-" + vpol.Name
		vapBindingName := constructBindingName(vapName)
		vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
		require.NoError(t, err)
		// 1 from policy + 1 from exception = 2 matchConditions
		require.Len(t, vap.Spec.MatchConditions, 2)
		require.Equal(t, "exp-1", vap.Spec.MatchConditions[1].Name)

		vap.ResourceVersion = "1"
		require.NoError(t, vapIndexer.Update(vap))
		binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), vapBindingName, metav1.GetOptions{})
		require.NoError(t, err)
		binding.ResourceVersion = "1"
		require.NoError(t, bindingIndexer.Update(binding))

		// 2. Expiry occurs: exception is now in the past
		polex.Spec.ExpiresAt = &metav1.Time{Time: now.Add(-time.Hour)}
		require.NoError(t, exceptionIndexer.Update(polex))

		// 3. Permission failure: RBAC is lost when timer fires
		auth.allowed = false
		err = c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name)
		require.Error(t, err, "reconciliation must return an error on permission failure to retain a retry")
		assert.Contains(t, err.Error(), "insufficient permissions")

		// Verify existing VAP still has the condition (not removed yet)
		vap, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, vap.Spec.MatchConditions, 2)

		// 4. Recovery: RBAC permissions are restored
		auth.allowed = true

		// 5. Retried reconciliation runs and removes the expired condition
		require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))

		vap, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
		require.NoError(t, err)
		// Only 1 matchCondition remains (the policy's own, "exp-1" has been removed)
		require.Len(t, vap.Spec.MatchConditions, 1)
		assert.Equal(t, "check-prod-label", vap.Spec.MatchConditions[0].Name)

		updatedVpol, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(context.Background(), vpol.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.True(t, updatedVpol.Status.Generated)
	})

	t.Run("MutatingPolicy", func(t *testing.T) {
		mpol := &policiesv1beta1.MutatingPolicy{
			TypeMeta: metav1.TypeMeta{Kind: "MutatingPolicy"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-map-perm-recover",
				UID:  "test-uid",
			},
			Spec: policiesv1beta1.MutatingPolicySpec{
				AutogenConfiguration: mapGenEnabled(),
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"configmaps"},
								},
							},
						},
					},
				},
				Mutations: []admissionregistrationv1alpha1.Mutation{
					{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{spec: Object.spec{containers: [Object.spec.containers[0]{name: "injected"}]}}`,
						},
					},
				},
			},
		}

		kubeClient := kubefake.NewSimpleClientset()
		kyvernoClient := versionedfake.NewSimpleClientset(mpol)
		queue := &delayRecorder{delays: map[any]time.Duration{}}
		exceptionIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		mapV1Indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		mapbindingV1Indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		require.NoError(t, mpolIndexer.Add(mpol))

		auth := &switchableAuthChecker{allowed: true}
		c := &controller{
			client:             kubeClient,
			kyvernoClient:      kyvernoClient,
			discoveryClient:    dclient.NewEmptyFakeClient().Discovery(),
			eventGen:           event.NewFake(),
			checker:            auth,
			queue:              queue,
			celpolexLister:     policiesv1beta1listers.NewPolicyExceptionLister(exceptionIndexer),
			mpolLister:         policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
			mapV1Lister:        admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapV1Indexer),
			mapbindingV1Lister: admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(mapbindingV1Indexer),
			polexEnabled:       true,
			exceptionNamespace: "*",
		}

		mapName := "mpol-" + mpol.Name
		mapBindingName := constructBindingName(mapName)

		// 1. Initial generation: active exception expiring in 1 hour
		now := time.Now()
		polex := expiringException("exp-map", mpol.Name, mpol.Kind, now.Add(time.Hour))
		require.NoError(t, exceptionIndexer.Add(polex))

		require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))

		mObj, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, mObj.Spec.MatchConditions, 1)
		require.Equal(t, "exp-map", mObj.Spec.MatchConditions[0].Name)

		mObj.ResourceVersion = "1"
		require.NoError(t, mapV1Indexer.Update(mObj))
		bObj, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(context.Background(), mapBindingName, metav1.GetOptions{})
		require.NoError(t, err)
		bObj.ResourceVersion = "1"
		require.NoError(t, mapbindingV1Indexer.Update(bObj))

		// 2. Expiry occurs
		polex.Spec.ExpiresAt = &metav1.Time{Time: now.Add(-time.Hour)}
		require.NoError(t, exceptionIndexer.Update(polex))

		// 3. Permission failure: RBAC is lost when timer fires
		auth.allowed = false
		err = c.reconcile(context.Background(), logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name)
		require.Error(t, err, "reconciliation must return an error on permission failure to retain a retry")
		assert.Contains(t, err.Error(), "insufficient permissions")

		// Verify existing MAP still has the condition
		mObj, err = kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Len(t, mObj.Spec.MatchConditions, 1)

		// 4. Recovery: RBAC permissions are restored
		auth.allowed = true

		// 5. Retried reconciliation runs and removes the expired condition
		require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))

		mObj, err = kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(context.Background(), mapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Empty(t, mObj.Spec.MatchConditions, "expired exception matchCondition must be removed after recovery")

		updatedMpol, err := kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(context.Background(), mpol.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.True(t, updatedMpol.Status.Generated)
	})
}
