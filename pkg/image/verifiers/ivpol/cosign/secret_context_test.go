package cosign

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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

func TestSignatureCredentialGETUsesRequestContext(t *testing.T) {

	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	client := &requestSecretClient{started: make(chan context.Context, 1)}
	lister := kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client)
	lister = kubeutils.ScopeSecretLister(lister, "tenant")
	authOpts, err := sourceRemoteOpts(lister, &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: "tenant/registry"}}})
	require.NoError(t, err)
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

func TestCheckOptionsPreservesSharedRemoteOptions(t *testing.T) {
	t.Parallel()
	backing := make([]remote.Option, 8)
	backing[0] = remote.WithContext(context.Background())
	baseOpts := backing[:1]
	attestor := &policiesv1beta1.Cosign{
		Key:   &policiesv1beta1.Key{Data: testPublicKey},
		CTLog: &policiesv1beta1.CTLog{InsecureIgnoreTlog: true},
		Source: &policiesv1beta1.Source{
			SignaturePullSecrets: []corev1.LocalObjectReference{{Name: "tenant/registry"}},
		},
	}
	const evaluations = 4
	start := make(chan struct{})
	finished := make(chan error, evaluations)
	for range evaluations {
		go func() {
			<-start
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := checkOptions(ctx, attestor, baseOpts, nil, nil)
			finished <- err
		}()
	}
	close(start)
	for range evaluations {
		require.NoError(t, <-finished)
	}
	for _, option := range backing[1:] {
		assert.Nil(t, option, "verification changed the shared option slice's spare capacity")
	}
}
