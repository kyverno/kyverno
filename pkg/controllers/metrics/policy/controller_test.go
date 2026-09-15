package policy

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"k8s.io/apimachinery/pkg/util/wait"
)

type fakeMetricsManager struct {
	metrics.MetricsConfigManager
}

func (m *fakeMetricsManager) Config() config.MetricsConfiguration {
	return config.NewDefaultMetricsConfiguration()
}

func (m *fakeMetricsManager) RecordPolicyChanges(ctx context.Context, policyValidationMode metrics.PolicyValidationMode, policyType metrics.PolicyType, policyBackgroundMode metrics.PolicyBackgroundMode, policyNamespace string, policyName string, policyChangeType string) {
}

func Test_MetricsInformerHandlers(t *testing.T) {
	metrics.SetManager(&fakeMetricsManager{})

	c := &controller{
		waitGroup: &wait.Group{},
	}

	cpol := &kyvernov1.ClusterPolicy{}
	pol := &kyvernov1.Policy{}

	c.addPolicy(cpol)
	c.updatePolicy(cpol, cpol)
	c.deletePolicy(cpol)

	c.addNsPolicy(pol)
	c.updateNsPolicy(pol, pol)
	c.deleteNsPolicy(pol)

	c.waitGroup.Wait()
}
