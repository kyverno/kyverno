package framework

import (
	"context"
	"sync"
	"testing"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	event "github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/utils/restmapper"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
)

// DelayedMpolProvider wraps an mpol provider so that a newly compiled policy version only becomes
// visible to the engine some time after the provider first returned it. Until then the engine sees
// the previously published version, or nothing for a new policy. This recreates the lag between the
// policy controller seeing a policy event and the engine having compiled it, which the synchronous
// framework wiring otherwise never produces.
//
// Only Fetch is delayed; MatchesMutateExisting (the admission path) is passed through.
type DelayedMpolProvider struct {
	inner mpolengine.Provider

	mu        sync.Mutex
	delay     time.Duration
	published map[string][]mpolengine.Policy
	pending   map[string]pendingMpolVersion
}

type pendingMpolVersion struct {
	policies []mpolengine.Policy
	seen     time.Time
}

func NewDelayedMpolProvider(inner mpolengine.Provider, delay time.Duration) *DelayedMpolProvider {
	return &DelayedMpolProvider{
		inner:     inner,
		delay:     delay,
		published: map[string][]mpolengine.Policy{},
		pending:   map[string]pendingMpolVersion{},
	}
}

// SetDelay changes the delay for versions compiled from now on. A zero delay publishes every
// version as soon as it is fetched.
func (p *DelayedMpolProvider) SetDelay(delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delay = delay
}

func (p *DelayedMpolProvider) Fetch(ctx context.Context, mutateExisting bool) []mpolengine.Policy {
	// Group by policy key: autogen variants of one policy are compiled, and published, together.
	current := map[string][]mpolengine.Policy{}
	for _, policy := range p.inner.Fetch(ctx, false) {
		key := mpolengine.PolicyKey(policy.Policy)
		current[key] = append(current[key], policy)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for key := range p.published {
		if _, ok := current[key]; !ok {
			delete(p.published, key)
		}
	}
	for key := range p.pending {
		if _, ok := current[key]; !ok {
			delete(p.pending, key)
		}
	}
	var out []mpolengine.Policy
	for key, policies := range current {
		// A recompile always produces a new compiled policy, including one triggered by a
		// PolicyException change that leaves the policy's generation alone.
		if published, ok := p.published[key]; !ok || published[0].CompiledPolicy != policies[0].CompiledPolicy {
			pending, ok := p.pending[key]
			if !ok || pending.policies[0].CompiledPolicy != policies[0].CompiledPolicy {
				pending = pendingMpolVersion{policies: policies, seen: now}
				p.pending[key] = pending
			}
			if now.Sub(pending.seen) >= p.delay {
				p.published[key] = pending.policies
				delete(p.pending, key)
			}
		}
		for _, policy := range p.published[key] {
			if !mutateExisting || policy.Policy.GetSpec().MutateExistingEnabled() {
				out = append(out, policy)
			}
		}
	}
	return out
}

func (p *DelayedMpolProvider) MatchesMutateExisting(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace *corev1.Namespace, requestMapFn func() (map[string]any, error)) []string {
	return p.inner.MatchesMutateExisting(ctx, attr, request, namespace, requestMapFn)
}

// NewMpolBackgroundController starts the background controller's UpdateRequest controller against
// envtest, wired like cmd/background-controller, so CELMutate UpdateRequests created in the
// kyverno namespace are processed by the real mpol processor with the real retry and backoff.
// It requires a TestEnv built with WithPolicyTypes(Mpol) and the kyverno.io CRDs.
//
// Each call builds its own dclient, so the controller starts on a cold discovery cache the way a
// freshly started background controller does. The controller stops, and any UpdateRequests left in
// the kyverno namespace are deleted, when the test ends.
func NewMpolBackgroundController(t *testing.T, env *TestEnv, eventGen event.Interface) {
	t.Helper()
	if env.Mpol == nil {
		t.Fatal("NewMpolBackgroundController requires a TestEnv wired with WithPolicyTypes(Mpol)")
	}
	ctx, cancel := context.WithCancel(context.Background())

	ensureNamespace(t, env, config.KyvernoNamespace())

	dynClient, err := dynamic.NewForConfig(env.Env.Config)
	if err != nil {
		cancel()
		t.Fatalf("failed to create dynamic client: %v", err)
	}
	dc, err := dclient.NewClient(ctx, dynClient, env.KubeClient, 15*time.Minute, false, nil)
	if err != nil {
		cancel()
		t.Fatalf("failed to create dclient: %v", err)
	}
	restMapper, err := restmapper.GetRESTMapper(dc)
	if err != nil {
		cancel()
		t.Fatalf("failed to create rest mapper: %v", err)
	}

	configuration := config.NewDefaultConfiguration(false)
	kubeInformer := kubeinformers.NewSharedInformerFactory(env.KubeClient, 0)
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(env.KyvernoClient, 0)
	controller := background.NewController(
		env.KyvernoClient,
		dc,
		nil, // the legacy engine only serves Mutate and Generate requests
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V2().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		env.ContextProvider,
		nil, // gpol engine, provider and watch manager only serve CELGenerate requests
		nil,
		nil,
		env.Mpol.Engine,
		restMapper,
		eventGen,
		configuration,
		jmespath.New(configuration),
		reportutils.NewReportingConfig(nil),
	)
	kubeInformer.Start(ctx.Done())
	kyvernoInformer.Start(ctx.Done())
	kubeInformer.WaitForCacheSync(ctx.Done())
	kyvernoInformer.WaitForCacheSync(ctx.Done())

	done := make(chan struct{})
	go func() {
		defer close(done)
		// genWorkers' default, so requests for one target can run concurrently as in production
		controller.Run(ctx, 10)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = env.KyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
	})
}

// SubmitCELMutateUR creates the bare CELMutate UpdateRequest the policy controller submits for a
// mutateExisting policy event or background scan (pkg/policy newCELMutateUR): created, then moved to
// Pending, which is what the background controller acts on. policyKey is the policy name, or
// namespace/name for a NamespacedMutatingPolicy.
func SubmitCELMutateUR(t *testing.T, env *TestEnv, policyKey string) {
	t.Helper()
	ctx := context.Background()
	urs := env.KyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace())
	created, err := urs.Create(ctx, &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "ur-", Namespace: config.KyvernoNamespace()},
		Spec:       kyvernov2.UpdateRequestSpec{Type: kyvernov2.CELMutate, Policy: policyKey},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create update request for %s: %v", policyKey, err)
	}
	created.Status.State = kyvernov2.Pending
	if _, err := urs.UpdateStatus(ctx, created, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to mark update request %s pending: %v", created.GetName(), err)
	}
}

// PendingURCount returns how many UpdateRequests are left in the kyverno namespace. The background
// controller deletes a request once it completes, so zero means every submitted request is done.
func PendingURCount(t *testing.T, env *TestEnv) int {
	t.Helper()
	list, err := env.KyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list update requests: %v", err)
	}
	return len(list.Items)
}

// ensureNamespace creates a namespace that may already exist, for namespaces shared by every test
// such as the kyverno namespace.
func ensureNamespace(t *testing.T, env *TestEnv, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if _, err := env.KubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create namespace %s: %v", name, err)
	}
}
