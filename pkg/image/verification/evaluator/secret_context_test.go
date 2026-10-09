package evaluator

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type requestSecretClient struct {
	corev1client.SecretsGetter
	started chan context.Context
}

type requestSecretNamespaceClient struct {
	corev1client.SecretInterface
	started chan context.Context
}

func (c *requestSecretClient) Secrets(string) corev1client.SecretInterface {
	return &requestSecretNamespaceClient{started: c.started}
}

func (c *requestSecretNamespaceClient) Get(ctx context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
	c.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

type requestForbiddenTransport struct{ calls int }

func (r *requestForbiddenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, errors.New("unexpected registry request")
}

func TestCompiledCredentialGETUsesRequestContext(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	client := &requestSecretClient{started: make(chan context.Context, 1)}
	lister := kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client)
	policy := &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "tenant"}, Spec: policiesv1beta1.ImageValidatingPolicySpec{Credentials: &policiesv1beta1.Credentials{Secrets: []string{"registry"}}}}
	compiled, errs := NewCompiler(lister).Compile(policy, nil)
	require.Empty(t, errs)
	authOpts := compiled.(*compiledPolicy).authOpts
	// Retaining the same compiled options must not retain either request's context.
	for range 2 {
		ctx, cancel := context.WithCancel(context.Background())
		transport := &requestForbiddenTransport{}
		options := append([]remote.Option{}, authOpts...)
		options = append(options, remote.WithTransport(transport), remote.WithContext(ctx))
		finished := make(chan error, 1)
		go func() {
			_, err := remote.Get(name.MustParseReference("192.0.2.1/image:latest"), options...)
			finished <- err
		}()
		select {
		case requestCtx := <-client.started:
			_, bounded := requestCtx.Deadline()
			assert.True(t, bounded)
		case <-time.After(time.Second):
			cancel()
			t.Fatal("live Secret GET did not start")
		}
		cancel()
		select {
		case err := <-finished:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("live Secret GET ignored registry request cancellation")
		}
		assert.Zero(t, transport.calls)
	}
}

func TestCompiledCredentialOptionsRejectMissingSecretBackend(t *testing.T) {
	libs.GetLibsCtx()
	t.Parallel()
	for _, namespace := range []string{"", "tenant"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			t.Parallel()
			spec := policiesv1beta1.ImageValidatingPolicySpec{Credentials: &policiesv1beta1.Credentials{Secrets: []string{"registry"}}}
			var policy policiesv1beta1.ImageValidatingPolicyLike = &policiesv1beta1.ImageValidatingPolicy{Spec: spec}
			if namespace != "" {
				policy = &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}, Spec: spec}
			}
			// Offline CLI evaluation compiles policies without a Kubernetes lister.
			compiled, errs := NewCompiler(nil).Compile(policy, nil)
			require.Empty(t, errs)
			transport := &requestForbiddenTransport{}
			options := append([]remote.Option{}, compiled.(*compiledPolicy).authOpts...)
			options = append(options, remote.WithTransport(transport), remote.WithContext(context.Background()))
			var err error
			require.NotPanics(t, func() {
				_, err = remote.Get(name.MustParseReference("192.0.2.1/image:latest"), options...)
			})
			require.ErrorContains(t, err, "secret lister is not configured")
			assert.Zero(t, transport.calls)
		})
	}
}
