package policy

import (
	"context"
	"sync"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

type controller struct {
	ruleInfo metrics.PolicyRuleMetrics

	// snapshot of the policies present in the cluster, kept up to date by the
	// event handlers so the metrics callback does not have to walk the informer
	// caches on every scrape
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
	if _, err := controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlers(polInformer.Informer(), c.addNsPolicy, c.updateNsPolicy, c.deleteNsPolicy); err != nil {
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
	for _, policy := range pols {
		err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer)
		if err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	for _, policy := range cpols {
		err := c.ruleInfo.RecordPolicyRuleInfo(ctx, policy, observer)
		if err != nil {
			logger.Error(err, "failed to report policy metric", "policy", policy)
			return err
		}
	}
	return nil
}

func (c *controller) startRoutine(routine func()) {
	c.waitGroup.Start(routine)
}

func (c *controller) addPolicy(obj interface{}) {
	p := obj.(*kyvernov1.ClusterPolicy)
	c.snapshot.upsert(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updatePolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.ClusterPolicy), cur.(*kyvernov1.ClusterPolicy)
	c.snapshot.upsert(curP)
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
	c.snapshot.upsert(p)
	// register kyverno_policy_changes_total metric concurrently
	c.startRoutine(func() { c.registerPolicyChangesMetricAddPolicy(context.TODO(), logger, p) })
}

func (c *controller) updateNsPolicy(old, cur interface{}) {
	oldP, curP := old.(*kyvernov1.Policy), cur.(*kyvernov1.Policy)
	c.snapshot.upsert(curP)
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

// policySnapshot holds the policies currently present in the cluster, keyed by
// namespace/name (or just name for cluster-wide policies). It is written by the
// informer event handlers and read by the metrics callback, which can run
// concurrently, so every access goes through the mutex.
type policySnapshot struct {
	mux   sync.RWMutex
	cpols map[string]kyvernov1.PolicyInterface
	pols  map[string]kyvernov1.PolicyInterface
}

func newPolicySnapshot() *policySnapshot {
	return &policySnapshot{
		cpols: make(map[string]kyvernov1.PolicyInterface),
		pols:  make(map[string]kyvernov1.PolicyInterface),
	}
}

func (s *policySnapshot) upsert(policy kyvernov1.PolicyInterface) {
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to compute policy key", "name", policy.GetName())
		return
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	if policy.GetNamespace() == "" {
		s.cpols[key] = policy
	} else {
		s.pols[key] = policy
	}
}

func (s *policySnapshot) remove(obj interface{}) {
	// deletion events may carry a DeletedFinalStateUnknown, in which case the
	// key it holds is the only way to identify the policy
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

func (s *policySnapshot) list() (cpols, pols []kyvernov1.PolicyInterface) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	cpols = make([]kyvernov1.PolicyInterface, 0, len(s.cpols))
	for _, policy := range s.cpols {
		cpols = append(cpols, policy)
	}
	pols = make([]kyvernov1.PolicyInterface, 0, len(s.pols))
	for _, policy := range s.pols {
		pols = append(pols, policy)
	}
	return cpols, pols
}
