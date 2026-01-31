package leaderelection

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNew_ValidParameters(t *testing.T) {
	logger := logr.Discard()
	name := "test-election"
	namespace := "test-namespace"
	kubeClient := fake.NewSimpleClientset()
	id := "test-instance-1"
	retryPeriod := DefaultRetryPeriod

	startWork := func(ctx context.Context) {
		// No-op for test
	}
	stopWork := func() {
		// No-op for test
	}

	le, err := New(logger, name, namespace, kubeClient, id, retryPeriod, startWork, stopWork)

	assert.NoError(t, err)
	assert.NotNil(t, le)
	assert.Equal(t, name, le.Name())
	assert.Equal(t, namespace, le.Namespace())
}

func TestNew_WithNilCallbacks(t *testing.T) {
	logger := logr.Discard()
	name := "test-election"
	namespace := "test-namespace"
	kubeClient := fake.NewSimpleClientset()
	id := "test-instance-2"
	retryPeriod := DefaultRetryPeriod

	// Test with nil callbacks
	le, err := New(logger, name, namespace, kubeClient, id, retryPeriod, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, le)
}

func TestNew_CustomRetryPeriod(t *testing.T) {
	tests := []struct {
		name        string
		retryPeriod time.Duration
	}{
		{
			name:        "default retry period",
			retryPeriod: DefaultRetryPeriod,
		},
		{
			name:        "custom short retry period",
			retryPeriod: 1 * time.Second,
		},
		{
			name:        "custom long retry period",
			retryPeriod: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logr.Discard()
			kubeClient := fake.NewSimpleClientset()

			le, err := New(logger, "test", "test-ns", kubeClient, "id", tt.retryPeriod, nil, nil)

			assert.NoError(t, err)
			assert.NotNil(t, le)
		})
	}
}

func TestConfig_Name(t *testing.T) {
	logger := logr.Discard()
	expectedName := "my-leader-election"
	kubeClient := fake.NewSimpleClientset()

	le, err := New(logger, expectedName, "test-ns", kubeClient, "id", DefaultRetryPeriod, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, expectedName, le.Name())
}

func TestConfig_Namespace(t *testing.T) {
	logger := logr.Discard()
	expectedNamespace := "my-namespace"
	kubeClient := fake.NewSimpleClientset()

	le, err := New(logger, "test", expectedNamespace, kubeClient, "id", DefaultRetryPeriod, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, expectedNamespace, le.Namespace())
}

func TestConfig_ID(t *testing.T) {
	logger := logr.Discard()
	expectedID := "unique-instance-id-123"
	kubeClient := fake.NewSimpleClientset()

	le, err := New(logger, "test", "test-ns", kubeClient, expectedID, DefaultRetryPeriod, nil, nil)

	assert.NoError(t, err)
	// ID should exactly match the identity we provided
	assert.Equal(t, expectedID, le.ID())
}

func TestConfig_IsLeader_InitiallyFalse(t *testing.T) {
	logger := logr.Discard()
	kubeClient := fake.NewSimpleClientset()

	le, err := New(logger, "test", "test-ns", kubeClient, "id", DefaultRetryPeriod, nil, nil)

	assert.NoError(t, err)
	// Initially should not be leader
	assert.False(t, le.IsLeader())
}

func TestConfig_IsLeader_AtomicBehavior(t *testing.T) {
	// Test that IsLeader uses atomic operations correctly
	cfg := &config{
		isLeader: 0,
	}

	// Initially not leader
	assert.False(t, cfg.IsLeader())

	// Simulate becoming leader
	atomic.StoreInt64(&cfg.isLeader, 1)
	assert.True(t, cfg.IsLeader())

	// Simulate losing leadership
	atomic.StoreInt64(&cfg.isLeader, 0)
	assert.False(t, cfg.IsLeader())
}

func TestConfig_GetLeader_BeforeElection(t *testing.T) {
	logger := logr.Discard()
	kubeClient := fake.NewSimpleClientset()

	le, err := New(logger, "test", "test-ns", kubeClient, "id", DefaultRetryPeriod, nil, nil)

	assert.NoError(t, err)
	// Before election starts, GetLeader returns empty string
	leader := le.GetLeader()
	assert.Empty(t, leader)
}

func TestConfig_MultipleInstances(t *testing.T) {
	// Test creating multiple leader election instances
	logger := logr.Discard()
	kubeClient := fake.NewSimpleClientset()

	instances := []struct {
		name      string
		namespace string
		id        string
	}{
		{"election-1", "ns-1", "instance-1"},
		{"election-2", "ns-2", "instance-2"},
		{"election-3", "ns-1", "instance-3"},
	}

	for _, inst := range instances {
		t.Run(inst.id, func(t *testing.T) {
			le, err := New(logger, inst.name, inst.namespace, kubeClient, inst.id, DefaultRetryPeriod, nil, nil)

			assert.NoError(t, err)
			assert.NotNil(t, le)
			assert.Equal(t, inst.name, le.Name())
			assert.Equal(t, inst.namespace, le.Namespace())
		})
	}
}

func TestNew_WithCallbacks(t *testing.T) {
	// Test that New succeeds with non-nil callbacks
	logger := logr.Discard()
	kubeClient := fake.NewSimpleClientset()

	startWork := func(ctx context.Context) {
		// Would be called when leadership is acquired
	}
	stopWork := func() {
		// Would be called when leadership is lost
	}

	le, err := New(logger, "test", "test-ns", kubeClient, "id", DefaultRetryPeriod, startWork, stopWork)

	assert.NoError(t, err)
	assert.NotNil(t, le)
}

func TestDefaultRetryPeriod(t *testing.T) {
	// Test that the default retry period constant is reasonable
	assert.Equal(t, 2*time.Second, DefaultRetryPeriod)
	assert.Greater(t, DefaultRetryPeriod, time.Duration(0))
}

func TestConfig_InterfaceCompliance(t *testing.T) {
	// Verify that config implements Interface
	var _ Interface = (*config)(nil)
}

func TestNew_DifferentNamespaces(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{"default namespace", "default"},
		{"kube-system namespace", "kube-system"},
		{"custom namespace", "my-app"},
		{"with dashes", "test-namespace-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logr.Discard()
			kubeClient := fake.NewSimpleClientset()

			le, err := New(logger, "test", tt.namespace, kubeClient, "id", DefaultRetryPeriod, nil, nil)

			assert.NoError(t, err)
			assert.Equal(t, tt.namespace, le.Namespace())
		})
	}
}

func TestNew_DifferentNames(t *testing.T) {
	tests := []struct {
		testName     string
		electionName string
	}{
		{"simple name", "leader"},
		{"with dashes", "my-leader-election"},
		{"with numbers", "election123"},
		{"long name", "very-long-leader-election-name-for-testing"},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			logger := logr.Discard()
			kubeClient := fake.NewSimpleClientset()

			le, err := New(logger, tt.electionName, "test-ns", kubeClient, "id", DefaultRetryPeriod, nil, nil)

			assert.NoError(t, err)
			assert.Equal(t, tt.electionName, le.Name())
		})
	}
}

func TestConfig_ConcurrentIsLeaderCalls(t *testing.T) {
	// Test that concurrent calls to IsLeader are safe (atomic operations)
	cfg := &config{
		isLeader: 0,
	}

	done := make(chan bool)

	// Multiple goroutines reading IsLeader
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = cfg.IsLeader()
			}
			done <- true
		}()
	}

	// One goroutine toggling leadership
	go func() {
		for i := 0; i < 100; i++ {
			atomic.StoreInt64(&cfg.isLeader, 1)
			atomic.StoreInt64(&cfg.isLeader, 0)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}

	// Test passes if no race conditions occur
}

// Note: Testing Run() would require starting actual leader election
// which involves timing, distributed state, and is better suited for
// integration tests rather than unit tests.
