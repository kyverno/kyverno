package imageverify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type runtimeSecretClient struct {
	corev1client.SecretsGetter
	started chan context.Context
}

type runtimeSecretNamespaceClient struct {
	corev1client.SecretInterface
	started chan context.Context
}

func (c *runtimeSecretClient) Secrets(string) corev1client.SecretInterface {
	return &runtimeSecretNamespaceClient{started: c.started}
}

func (c *runtimeSecretNamespaceClient) Get(ctx context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
	c.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

type runtimeForbiddenTransport struct{}

func (runtimeForbiddenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected registry request")
}

type runtimeSuccessfulImages struct{ imagedataloader.ImageContext }

func (runtimeSuccessfulImages) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	image := &imagedataloader.ImageData{}
	image.AddVerifiedIntotoPayloads("proof", []byte(`{"valid":true}`))
	return image, nil
}

func TestRuntimeSecretGETUsesEvaluationContext(t *testing.T) {
	t.Parallel()
	expressions := []string{
		`verifyImageSignatures("192.0.2.1/image:latest", [])`,
		`verifyAttestationSignatures("192.0.2.1/image:latest", "proof", [])`,
		`getImageData("192.0.2.1/image:latest")`,
		`extractPayload("192.0.2.1/image:latest", "proof")`,
	}
	for i, expression := range expressions {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			client := &runtimeSecretClient{started: make(chan context.Context, 1)}
			lister := kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client)
			policy := &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "tenant"}, Spec: policiesv1beta1.ImageValidatingPolicySpec{
				Credentials:  &policiesv1beta1.Credentials{Secrets: []string{"registry"}},
				Attestations: []policiesv1beta1.Attestation{{Name: "proof", InToto: &policiesv1beta1.InToto{Type: "proof"}}},
			}}
			env, err := cel.NewEnv(Lib(nil, nil, policy, lister, logr.Discard(), nil, nil), ext.NativeTypes(reflect.TypeFor[imagedataloader.ImageData]()))
			require.NoError(t, err)
			functions, err := ImageVerifyCELFuncs(logr.Discard(), nil, policy, lister, nil, env.CELTypeAdapter(), nil)
			require.NoError(t, err)
			functions.authOpts = append(functions.authOpts, remote.WithTransport(runtimeForbiddenTransport{}))
			ast, issues := env.Compile(expression)
			require.NoError(t, issues.Err())
			program, err := env.Program(ast)
			require.NoError(t, err)
			for range 2 {
				images, err := imagedataloader.NewImageContext(nil, nil, nil)
				require.NoError(t, err)
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				bound := NewRuntimeForPolicy(functions, images, nil, NewImageVerificationResults()).WithContext(ctx)
				finished := make(chan error, 1)
				go func() {
					_, _, err := program.ContextEval(ctx, map[string]any{RuntimeKey: bound})
					finished <- err
				}()
				select {
				case requestCtx := <-client.started:
					deadline, ok := requestCtx.Deadline()
					assert.True(t, ok)
					assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
				case <-time.After(time.Second):
					cancel()
					t.Fatal("runtime Secret GET did not start")
				}
				// Reused compiled functions must keep other requests independent.
				var concurrent sync.WaitGroup
				for range 2 {
					concurrent.Go(func() {
						successCtx := context.Background()
						runtime := NewRuntimeForPolicy(functions, runtimeSuccessfulImages{}, nil, NewImageVerificationResults()).WithContext(successCtx)
						_, _, err := program.ContextEval(successCtx, map[string]any{RuntimeKey: runtime})
						assert.NoError(t, err)
					})
				}
				concurrent.Wait()
				cancel()
				select {
				case err := <-finished:
					require.ErrorContains(t, err, "context canceled")
				case <-time.After(time.Second):
					t.Fatal("runtime Secret GET ignored evaluation cancellation")
				}
			}
		})
	}
}
