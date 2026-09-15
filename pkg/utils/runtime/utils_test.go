package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

type mockCertValidator struct {
	valid bool
	err   error
}

func (m *mockCertValidator) ValidateCert(ctx context.Context) (bool, error) {
	return m.valid, m.err
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

	rValid := NewRuntime(logger, "", deploymentInformer, &mockCertValidator{valid: true, err: nil})
	assert.True(t, rValid.IsReady(context.TODO()))

	rInvalid := NewRuntime(logger, "", deploymentInformer, &mockCertValidator{valid: false, err: nil})
	assert.False(t, rInvalid.IsReady(context.TODO()))

	rErr := NewRuntime(logger, "", deploymentInformer, &mockCertValidator{valid: false, err: errors.New("test err")})
	assert.False(t, rErr.IsReady(context.TODO()))
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
	deploymentInformer.Informer().GetStore().Add(deployment)
	assert.False(t, r.IsRollingUpdate())

	deployment.Status.Replicas = 2
	deploymentInformer.Informer().GetStore().Update(deployment)
	assert.True(t, r.IsRollingUpdate())
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
	deploymentInformer.Informer().GetStore().Add(deployment)
	assert.False(t, r.IsGoingDown())

	now := metav1.NewTime(time.Now())
	deployment.SetDeletionTimestamp(&now)
	deploymentInformer.Informer().GetStore().Update(deployment)
	assert.True(t, r.IsGoingDown())
	deployment.SetDeletionTimestamp(nil)

	zero := int32(0)
	deployment.Spec.Replicas = &zero
	deploymentInformer.Informer().GetStore().Update(deployment)
	assert.True(t, r.IsGoingDown())
}
