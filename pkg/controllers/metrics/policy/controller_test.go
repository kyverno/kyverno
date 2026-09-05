package policy

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestController(t *testing.T) {
	metrics.InitMetrics(
		context.TODO(),
		true,
		"none",
		0,
		"",
		config.NewDefaultMetricsConfiguration(),
		"",
		nil,
		nil,
		nil,
		"",
		"",
		"",
		logr.Discard(),
	)

	client := kyvernoclient.NewSimpleClientset()
	informers := kyvernoinformer.NewSharedInformerFactory(client, 0)
	cpolInformer := informers.Kyverno().V1().ClusterPolicies()
	polInformer := informers.Kyverno().V1().Policies()

	ctrl := NewController(cpolInformer, polInformer)
	c := ctrl.(*controller)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ctrl.Run(ctx, 1)

	// Wait for worker to start
	time.Sleep(100 * time.Millisecond)

	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cpol",
		},
	}
	pol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pol",
			Namespace: "default",
		},
	}

	// Test ClusterPolicy Add
	c.addPolicy(cpol)

	// Test ClusterPolicy Update
	cpolUpdated := cpol.DeepCopy()
	cpolUpdated.Annotations = map[string]string{"foo": "bar"}
	c.updatePolicy(cpol, cpolUpdated)

	// Test ClusterPolicy Delete
	c.deletePolicy(cpol)

	// Test Policy Add
	c.addNsPolicy(pol)

	// Test Policy Update
	polUpdated := pol.DeepCopy()
	polUpdated.Annotations = map[string]string{"foo": "bar"}
	c.updateNsPolicy(pol, polUpdated)

	// Test Policy Delete
	c.deleteNsPolicy(pol)

	// Give the queue time to process
	time.Sleep(200 * time.Millisecond)

	// Ensure queue is empty
	if c.queue.Len() != 0 {
		t.Errorf("Expected queue to be empty, got %d", c.queue.Len())
	}
}
