package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// mockCertValidator implements tls.CertValidator for testing
type mockCertValidator struct {
	validity bool
	err      error
}

func (m *mockCertValidator) ValidateCert(ctx context.Context) (bool, error) {
	return m.validity, m.err
}

func TestNewRuntime(t *testing.T) {
	logger := logr.Discard()
	serverIP := "127.0.0.1"

	// Create fake client and informer
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	certValidator := &mockCertValidator{validity: true, err: nil}

	// Create runtime
	rt := NewRuntime(logger, serverIP, deploymentInformer, certValidator)

	// Assertions
	assert.NotNil(t, rt)
}

func TestRuntime_IsDebug_WithServerIP(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	rt := NewRuntime(logr.Discard(), "127.0.0.1", deploymentInformer, &mockCertValidator{})

	assert.True(t, rt.IsDebug())
}

func TestRuntime_IsDebug_WithoutServerIP(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	assert.False(t, rt.IsDebug())
}

func TestRuntime_IsLive_AlwaysTrue(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})
	ctx := context.Background()

	// IsLive should always return true
	assert.True(t, rt.IsLive(ctx))
}

func TestRuntime_IsReady_ValidCertificates(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	certValidator := &mockCertValidator{validity: true, err: nil}
	rt := NewRuntime(logr.Discard(), "", deploymentInformer, certValidator)
	ctx := context.Background()

	assert.True(t, rt.IsReady(ctx))
}

func TestRuntime_IsReady_InvalidCertificates(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	certValidator := &mockCertValidator{validity: false, err: nil}
	rt := NewRuntime(logr.Discard(), "", deploymentInformer, certValidator)
	ctx := context.Background()

	assert.False(t, rt.IsReady(ctx))
}

func TestRuntime_IsReady_CertValidationError(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	certValidator := &mockCertValidator{validity: false, err: assert.AnError}
	rt := NewRuntime(logr.Discard(), "", deploymentInformer, certValidator)
	ctx := context.Background()

	assert.False(t, rt.IsReady(ctx))
}

func TestRuntime_IsRollingUpdate_InDebugMode(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Debug mode (serverIP set)
	rt := NewRuntime(logr.Discard(), "127.0.0.1", deploymentInformer, &mockCertValidator{})

	// In debug mode, IsRollingUpdate should return false
	assert.False(t, rt.IsRollingUpdate())
}

func TestRuntime_IsRollingUpdate_NormalReplicas(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-admission-controller",
			Namespace: "kyverno",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 3, // Same as desired
		},
	}

	kubeClient := fake.NewSimpleClientset(deployment)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer and wait for cache sync
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Not rolling update
	assert.False(t, rt.IsRollingUpdate())
}

func TestRuntime_IsRollingUpdate_ExtraReplicas(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-admission-controller",
			Namespace: "kyverno",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 5, // More than desired - rolling update in progress
		},
	}

	kubeClient := fake.NewSimpleClientset(deployment)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer and wait for cache sync
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Is rolling update
	assert.True(t, rt.IsRollingUpdate())
}

func TestRuntime_IsGoingDown_InDebugMode(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Debug mode (serverIP set)
	rt := NewRuntime(logr.Discard(), "127.0.0.1", deploymentInformer, &mockCertValidator{})

	// In debug mode, IsGoingDown should return false
	assert.False(t, rt.IsGoingDown())
}

func TestRuntime_IsGoingDown_DeploymentNotFound(t *testing.T) {
	// No deployment exists
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Deployment not found means going down
	assert.True(t, rt.IsGoingDown())
}

func TestRuntime_IsGoingDown_WithDeletionTimestamp(t *testing.T) {
	now := metav1.Now()
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "kyverno-admission-controller",
			Namespace:         "kyverno",
			DeletionTimestamp: &now,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	kubeClient := fake.NewSimpleClientset(deployment)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer and wait for cache sync
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Deployment being deleted
	assert.True(t, rt.IsGoingDown())
}

func TestRuntime_IsGoingDown_ZeroReplicas(t *testing.T) {
	replicas := int32(0)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-admission-controller",
			Namespace: "kyverno",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	kubeClient := fake.NewSimpleClientset(deployment)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer and wait for cache sync
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Zero replicas means going down
	assert.True(t, rt.IsGoingDown())
}

func TestRuntime_IsGoingDown_NormalState(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-admission-controller",
			Namespace: "kyverno",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	kubeClient := fake.NewSimpleClientset(deployment)
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	// Start informer and wait for cache sync
	stop := make(chan struct{})
	defer close(stop)
	informerFactory.Start(stop)
	informerFactory.WaitForCacheSync(stop)

	rt := NewRuntime(logr.Discard(), "", deploymentInformer, &mockCertValidator{})

	// Normal state - not going down
	assert.False(t, rt.IsGoingDown())
}

func TestRuntime_InterfaceCompliance(t *testing.T) {
	// Verify that runtime implements Runtime interface
	var _ Runtime = (*runtime)(nil)
}

func TestRuntime_ContextPropagation(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	certValidator := &mockCertValidator{validity: true, err: nil}
	rt := NewRuntime(logr.Discard(), "", deploymentInformer, certValidator)

	// Test with different contexts
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "background context",
			ctx:  context.Background(),
		},
		{
			name: "context with timeout",
			ctx:  func() context.Context { ctx, _ := context.WithTimeout(context.Background(), time.Second); return ctx }(),
		},
		{
			name: "context with value",
			ctx:  context.WithValue(context.Background(), "key", "value"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// IsReady should work with different context types
			result := rt.IsReady(tt.ctx)
			assert.True(t, result)

			// IsLive should work with different context types
			liveResult := rt.IsLive(tt.ctx)
			assert.True(t, liveResult)
		})
	}
}
