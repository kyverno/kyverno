package policy

import (
	"context"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	// series contributed by the policies present in the cluster, computed once
	// per policy event and replayed by the metrics callback on every scrape
	snapshot *policySnapshot

	waitGroup *wait.Group
}

// TODO: this is a strange controller, it only processes events, this should be changed to a real controller.
func NewController(
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	waitGroup *wait.Group,
) {
	c := controller{
		ruleInfo:  metrics.GetPolicyInfoMetrics(),
		snapshot:  newPolicySnapshot(),
		waitGroup: waitGroup,
	}
	// handlers are registered directly instead of through controllerutils.AddEventHandlers,
	// whose delete wrapper discards the DeletedFinalStateUnknown tombstone; the delete
	// handlers below need the tombstone key to drop a policy from the snapshot when the
	// deleted object is no longer available
	if _, err := cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addPolicy,
		UpdateFunc: c.updatePolicy,
		DeleteFunc: c.deletePolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addNsPolicy,
		UpdateFunc: c.updateNsPolicy,
		DeleteFunc: c.deleteNsPolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if c.ruleInfo != nil {
		_, err := c.ruleInfo.RegisterCallback(c.report)
		if err != nil {
			logger.Error(err, "Failed to register callback")
		}
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	if c.ruleInfo == nil {
		return nil
	}
	cpols, pols := c.snapshot.list()
	for _, obs := range pols {
		c.ruleInfo.RecordPolicyRuleObservations(obs, observer)
	}
	for _, obs := range cpols {
		c.ruleInfo.RecordPolicyRuleObservations(obs, observer)
	}
	return nil
}

func (c *controller) buildObservations(policy kyvernov1.PolicyInterface) metrics.PolicyRuleObservation {
	if c.ruleInfo == nil {
		return nil
	}
	obs, err := c.ruleInfo.BuildPolicyRuleObservations(policy)
	if err != nil {
		logger.Error(err, "failed to compute policy rule metric", "name", policy.GetName())
		return nil
	}
	return obs
}

// storeObservations refreshes the snapshot entry of a policy. A nil observation
// means the policy contributes no series anymore, either because its build
// failed or because the metrics configuration filters its namespace out, so any
// entry it may have is dropped.
func (c *controller) storeObservations(policy kyvernov1.PolicyInterface) {
	obs := c.buildObservations(policy)
	if obs == nil {
		c.snapshot.remove(policy)
		return
	}
	c.snapshot.upsert(policy, obs)
}

func (c *controller) startRoutine(routine func()) {
	c.waitGroup.Start(routine)
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.storeObservations(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	c.storeObservations(curP)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deletePolicy(obj interface{}) {
	c.snapshot.remove(obj)
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}

func (c *controller) addNsPolicy(obj interface{}) {
	p := obj.(*kyvernov1.Policy)
	c.storeObservations(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	c.storeObservations(curP)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricUpdatePolicy(context.TODO(), logger, oldP, curP) })
}

func (c *controller) deleteNsPolicy(obj interface{}) {
	c.snapshot.remove(obj)
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricDeletePolicy(context.TODO(), logger, p) })
}

// policySnapshot holds the series contributed by the policies currently present
// in the cluster, keyed by namespace/name (or just name for cluster-wide
// policies). It is written by the informer event handlers and read by the
// metrics callback, which can run concurrently, so every access goes through
// the mutex.
type policySnapshot struct {
	mux   sync.RWMutex
	cpols map[string]metrics.PolicyRuleObservation
	pols  map[string]metrics.PolicyRuleObservation
}

func newPolicySnapshot() *policySnapshot {
	return &policySnapshot{
		cpols: make(map[string]metrics.PolicyRuleObservation),
		pols:  make(map[string]metrics.PolicyRuleObservation),
	}
}

func (s *policySnapshot) upsert(policy kyvernov1.PolicyInterface, obs metrics.PolicyRuleObservation) {
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to compute policy key", "name", policy.GetName())
		return
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	if policy.GetNamespace() == "" {
		s.cpols[key] = obs
	} else {
		s.pols[key] = obs
	}
}

func (s *policySnapshot) remove(obj interface{}) {
	// delete events may carry a cache.DeletedFinalStateUnknown, in which case
	// the key it holds is the only way to identify the policy
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to compute policy key")
		return
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.cpols, key)
	delete(s.pols, key)
}

func (s *policySnapshot) list() (cpols, pols []metrics.PolicyRuleObservation) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	cpols = make([]metrics.PolicyRuleObservation, 0, len(s.cpols))
	for _, obs := range s.cpols {
		cpols = append(cpols, obs)
	}
	pols = make([]metrics.PolicyRuleObservation, 0, len(s.pols))
	for _, obs := range s.pols {
		pols = append(pols, obs)
	}
	return cpols, pols
}
