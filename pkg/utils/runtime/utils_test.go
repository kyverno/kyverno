package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type fakeCertValidator struct {
	valid bool
	err   error
}

func (f fakeCertValidator) ValidateCert(context.Context) (bool, error) {
	return f.valid, f.err
}

// newTestRuntime builds a runtime backed by an in-memory lister. Passing a nil
// deployment leaves the lister empty, which makes getDeployment return a
// NotFound error.
func newTestRuntime(t *testing.T, serverIP string, deployment *appsv1.Deployment) *runtime {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if deployment != nil {
		require.NoError(t, indexer.Add(deployment))
	}
	return &runtime{
		serverIP:         serverIP,
		deploymentLister: appsv1listers.NewDeploymentLister(indexer),
		logger:           logr.Discard(),
	}
}

// newDeployment builds a deployment named and namespaced the way getDeployment
// looks it up.
func newDeployment(specReplicas *int32, statusReplicas int32, deleted bool) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.KyvernoDeploymentName(),
			Namespace: config.KyvernoNamespace(),
		},
		Spec:   appsv1.DeploymentSpec{Replicas: specReplicas},
		Status: appsv1.DeploymentStatus{Replicas: statusReplicas},
	}
	if deleted {
		now := metav1.Now()
		deployment.DeletionTimestamp = &now
	}
	return deployment
}

func ptr(i int32) *int32 {
	return &i
}

func TestIsDebug(t *testing.T) {
	assert.True(t, newTestRuntime(t, "127.0.0.1", nil).IsDebug())
	assert.False(t, newTestRuntime(t, "", nil).IsDebug())
}

func TestIsLive(t *testing.T) {
	assert.True(t, newTestRuntime(t, "", nil).IsLive(context.Background()))
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name      string
		validator fakeCertValidator
		want      bool
	}{
		{name: "valid certificates", validator: fakeCertValidator{valid: true}, want: true},
		{name: "invalid certificates", validator: fakeCertValidator{valid: false}, want: false},
		{
			name:      "validation error is not ready",
			validator: fakeCertValidator{valid: true, err: errors.New("boom")},
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestRuntime(t, "", nil)
			c.certValidator = tt.validator
			assert.Equal(t, tt.want, c.IsReady(context.Background()))
		})
	}
}

func TestIsRollingUpdate(t *testing.T) {
	tests := []struct {
		name       string
		serverIP   string
		deployment *appsv1.Deployment
		want       bool
	}{
		{
			name:     "debug mode never reports a rolling update",
			serverIP: "127.0.0.1",
			want:     false,
		},
		{
			name: "missing deployment is treated as a rolling update",
			want: true,
		},
		{
			name:       "more non terminated replicas than desired",
			deployment: newDeployment(ptr(2), 3, false),
			want:       true,
		},
		{
			name:       "replica counts match",
			deployment: newDeployment(ptr(2), 2, false),
			want:       false,
		},
		{
			name:       "fewer non terminated replicas than desired",
			deployment: newDeployment(ptr(3), 1, false),
			want:       false,
		},
		{
			name:       "nil spec replicas defaults to one and detects the update",
			deployment: newDeployment(nil, 2, false),
			want:       true,
		},
		{
			name:       "nil spec replicas defaults to one and stays quiet",
			deployment: newDeployment(nil, 1, false),
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestRuntime(t, tt.serverIP, tt.deployment)
			assert.Equal(t, tt.want, c.IsRollingUpdate())
		})
	}
}

func TestIsGoingDown(t *testing.T) {
	tests := []struct {
		name       string
		serverIP   string
		deployment *appsv1.Deployment
		want       bool
	}{
		{
			name:     "debug mode never reports going down",
			serverIP: "127.0.0.1",
			want:     false,
		},
		{
			name: "missing deployment reports going down",
			want: true,
		},
		{
			name:       "deployment with a deletion timestamp",
			deployment: newDeployment(ptr(1), 1, true),
			want:       true,
		},
		{
			name:       "scaled to zero",
			deployment: newDeployment(ptr(0), 0, false),
			want:       true,
		},
		{
			name:       "running normally",
			deployment: newDeployment(ptr(1), 1, false),
			want:       false,
		},
		{
			name:       "nil spec replicas is not going down",
			deployment: newDeployment(nil, 1, false),
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestRuntime(t, tt.serverIP, tt.deployment)
			assert.Equal(t, tt.want, c.IsGoingDown())
		})
	}
}

func TestNewRuntime(t *testing.T) {
	informer := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Apps().V1().Deployments()
	c := NewRuntime(logr.Discard(), "127.0.0.1", informer, fakeCertValidator{valid: true})
	require.NotNil(t, c)
	assert.True(t, c.IsDebug())
	assert.True(t, c.IsReady(context.Background()))
}
