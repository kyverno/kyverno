package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
)

type certValidator struct {
	valid bool
	err   error
}

func (c certValidator) ValidateCert(context.Context) (bool, error) { return c.valid, c.err }

type mockDeploymentLister struct {
	appsv1listers.DeploymentLister
	err error
}

func (m *mockDeploymentLister) Deployments(namespace string) appsv1listers.DeploymentNamespaceLister {
	return &mockDeploymentNamespaceLister{err: m.err}
}

type mockDeploymentNamespaceLister struct {
	appsv1listers.DeploymentNamespaceLister
	err error
}

func (m *mockDeploymentNamespaceLister) Get(name string) (*appsv1.Deployment, error) {
	return nil, m.err
}

func TestReadinessAndLiveness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cert   certValidator
		checks []func() bool
		code   int
	}{
		{"existing caller", certValidator{valid: true}, nil, http.StatusOK},
		{"healthy informer", certValidator{valid: true}, []func() bool{func() bool { return true }}, http.StatusOK},
		{"unhealthy informer", certValidator{valid: true}, []func() bool{func() bool { return false }}, http.StatusInternalServerError},
		{"invalid certificate", certValidator{}, []func() bool{func() bool { return true }}, http.StatusInternalServerError},
		{"certificate error", certValidator{err: errors.New("unavailable")}, nil, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &runtime{logger: logr.Discard(), certValidator: tc.cert, readinessChecks: tc.checks}
			out := httptest.NewRecorder()
			handlers.Probe(r.IsReady)(out, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
			require.Equal(t, tc.code, out.Code)
			require.True(t, r.IsLive(t.Context()))
		})
	}
}

func TestRuntime_IsDebug(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	logger := testr.New(t)

	r := NewRuntime(logger, "127.0.0.1", deploymentInformer, nil)
	assert.True(t, r.IsDebug())

	r2 := NewRuntime(logger, "", deploymentInformer, nil)
	assert.False(t, r2.IsDebug())
}

func TestRuntime_IsLive(t *testing.T) {
	r := &runtime{}
	assert.True(t, r.IsLive(context.TODO()))
}

func TestRuntime_IsReady(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	logger := testr.New(t)

	rValid := NewRuntime(logger, "", deploymentInformer, certValidator{valid: true})
	assert.True(t, rValid.IsReady(context.TODO()))

	rInvalid := NewRuntime(logger, "", deploymentInformer, certValidator{valid: false})
	assert.False(t, rInvalid.IsReady(context.TODO()))

	rErr := NewRuntime(logger, "", deploymentInformer, certValidator{valid: true, err: errors.New("test err")})
	assert.False(t, rErr.IsReady(context.TODO()))

	rChecks := NewRuntime(logger, "", deploymentInformer, certValidator{valid: true}, func() bool { return false })
	assert.False(t, rChecks.IsReady(context.TODO()))
}

func TestRuntime_IsRollingUpdate(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	logger := testr.New(t)

	r := NewRuntime(logger, "", deploymentInformer, nil)

	rDebug := NewRuntime(logger, "127.0.0.1", deploymentInformer, nil)
	assert.False(t, rDebug.IsRollingUpdate())

	assert.True(t, r.IsRollingUpdate())

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.KyvernoDeploymentName(),
			Namespace: config.KyvernoNamespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 1,
		},
	}
	assert.NoError(t, deploymentInformer.Informer().GetStore().Add(deployment))
	assert.False(t, r.IsRollingUpdate())

	updated := deployment.DeepCopy()
	updated.Status.Replicas = 2
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(updated))
	assert.True(t, r.IsRollingUpdate())

	nilReplicasDep := deployment.DeepCopy()
	nilReplicasDep.Spec.Replicas = nil
	nilReplicasDep.Status.Replicas = 1
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(nilReplicasDep))
	assert.False(t, r.IsRollingUpdate())

	nilReplicasDepRolling := nilReplicasDep.DeepCopy()
	nilReplicasDepRolling.Status.Replicas = 2
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(nilReplicasDepRolling))
	assert.True(t, r.IsRollingUpdate())

	rGenericErr := &runtime{logger: logger, deploymentLister: &mockDeploymentLister{err: errors.New("lister error")}}
	assert.True(t, rGenericErr.IsRollingUpdate())
}

func TestRuntime_IsGoingDown(t *testing.T) {
	client := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	logger := testr.New(t)

	r := NewRuntime(logger, "", deploymentInformer, nil)

	rDebug := NewRuntime(logger, "127.0.0.1", deploymentInformer, nil)
	assert.False(t, rDebug.IsGoingDown())

	assert.True(t, r.IsGoingDown())

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.KyvernoDeploymentName(),
			Namespace: config.KyvernoNamespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
	assert.NoError(t, deploymentInformer.Informer().GetStore().Add(deployment))
	assert.False(t, r.IsGoingDown())

	now := metav1.NewTime(time.Now())
	updatedDeletion := deployment.DeepCopy()
	updatedDeletion.SetDeletionTimestamp(&now)
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(updatedDeletion))
	assert.True(t, r.IsGoingDown())

	zero := int32(0)
	updatedZeroReplicas := deployment.DeepCopy()
	updatedZeroReplicas.Spec.Replicas = &zero
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(updatedZeroReplicas))
	assert.True(t, r.IsGoingDown())

	nilReplicasDep := deployment.DeepCopy()
	nilReplicasDep.Spec.Replicas = nil
	assert.NoError(t, deploymentInformer.Informer().GetStore().Update(nilReplicasDep))
	assert.False(t, r.IsGoingDown())

	rGenericErr := &runtime{logger: logger, deploymentLister: &mockDeploymentLister{err: errors.New("lister error")}}
	assert.False(t, rGenericErr.IsGoingDown())
}
