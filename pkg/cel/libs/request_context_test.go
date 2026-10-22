package libs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// countingFetcher counts fetches that reach the (real) image data loader.
type countingFetcher struct {
	imagedataloader.Fetcher
	calls atomic.Int64
}

func (f *countingFetcher) FetchImageData(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	f.calls.Add(1)
	return f.Fetcher.FetchImageData(ctx, image, authOpts, nameOpts)
}

type testRegistry struct {
	host string
	// delay holds manifest requests for this long, or until the client goes away
	delay atomic.Int64
}

func newTestRegistry(t *testing.T, images ...string) *testRegistry {
	t.Helper()
	reg := &testRegistry{}
	backend := registry.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d := time.Duration(reg.delay.Load()); d > 0 && strings.Contains(r.URL.Path, "/manifests/") {
			select {
			case <-time.After(d):
			case <-r.Context().Done():
				return
			}
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	reg.host = strings.TrimPrefix(server.URL, "http://")
	for _, image := range images {
		img, err := random.Image(64, 1)
		require.NoError(t, err)
		ref, err := name.ParseReference(reg.host + "/" + image)
		require.NoError(t, err)
		require.NoError(t, remote.Write(ref, img))
	}
	return reg
}

func newTestProvider(t *testing.T) (*contextProvider, *countingFetcher) {
	t.Helper()
	loader, err := imagedataloader.New(nil, nil, nil)
	require.NoError(t, err)
	fetcher := &countingFetcher{Fetcher: loader}
	return &contextProvider{imagedata: fetcher}, fetcher
}

func TestRequestContext_GetImageData_HonorsDeadline(t *testing.T) {
	reg := newTestRegistry(t, "app:v1")
	provider, _ := newTestProvider(t)
	image := reg.host + "/app:v1"

	t.Run("control without deadline succeeds", func(t *testing.T) {
		data, err := WithRequestContext(context.Background(), provider).GetImageData(image, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("deadline cancels a slow registry", func(t *testing.T) {
		reg.delay.Store(int64(5 * time.Second))
		defer reg.delay.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := WithRequestContext(ctx, provider).GetImageData(image, nil)
		require.Error(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("cancelled context fails fast", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := WithRequestContext(ctx, provider).GetImageData(image, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func newImageEnv(t *testing.T, global Context, authOpts []remote.Option) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(imagedata.Lib(imagedata.Context{ContextInterface: global}, imagedata.Latest(), authOpts))
	require.NoError(t, err)
	return env
}

func evalExpr(t *testing.T, env *cel.Env, ctx context.Context, expr string, activation map[string]any) error {
	t.Helper()
	ast, issues := env.Compile(expr)
	require.NoError(t, issues.Err())
	prg, err := env.Program(ast, cel.InterruptCheckFrequency(1))
	require.NoError(t, err)
	_, _, err = prg.ContextEval(ctx, activation)
	return err
}

func TestRequestContext_GetMetadata_MemoizedPerEvaluation(t *testing.T) {
	reg := newTestRegistry(t, "a:v1", "b:v1")
	provider, fetcher := newTestProvider(t)
	// the compile-time global is the raw provider, as in the policy compilers
	env := newImageEnv(t, provider, nil)
	a, b := reg.host+"/a:v1", reg.host+"/b:v1"
	bind := func(ctx context.Context) map[string]any {
		activation := map[string]any{}
		BindRequestContext(ctx, provider, activation)
		return activation
	}
	fourCalls := `image.GetMetadata("` + a + `").manifest != null && image.GetMetadata("` + a + `").manifest != null && image.GetMetadata("` + a + `").manifest != null && image.GetMetadata("` + a + `").manifest != null`

	t.Run("without binding every call fetches", func(t *testing.T) {
		fetcher.calls.Store(0)
		require.NoError(t, evalExpr(t, env, context.Background(), fourCalls, nil))
		assert.EqualValues(t, 4, fetcher.calls.Load())
	})
	t.Run("same image four times is one fetch", func(t *testing.T) {
		fetcher.calls.Store(0)
		require.NoError(t, evalExpr(t, env, context.Background(), fourCalls, bind(context.Background())))
		assert.EqualValues(t, 1, fetcher.calls.Load())
	})
	t.Run("distinct images are fetched separately", func(t *testing.T) {
		fetcher.calls.Store(0)
		expr := `image.GetMetadata("` + a + `").manifest != null && image.GetMetadata("` + b + `").manifest != null && image.GetMetadata("` + a + `").manifest != null`
		require.NoError(t, evalExpr(t, env, context.Background(), expr, bind(context.Background())))
		assert.EqualValues(t, 2, fetcher.calls.Load())
	})
	t.Run("separate evaluations do not share results", func(t *testing.T) {
		fetcher.calls.Store(0)
		require.NoError(t, evalExpr(t, env, context.Background(), fourCalls, bind(context.Background())))
		require.NoError(t, evalExpr(t, env, context.Background(), fourCalls, bind(context.Background())))
		assert.EqualValues(t, 2, fetcher.calls.Load())
	})
	t.Run("errors are memoized within an evaluation only", func(t *testing.T) {
		fetcher.calls.Store(0)
		missing := reg.host + "/missing:v1"
		expr := `image.GetMetadata("` + missing + `").manifest != null`
		activation := bind(context.Background())
		require.Error(t, evalExpr(t, env, context.Background(), expr, activation))
		require.Error(t, evalExpr(t, env, context.Background(), expr, activation))
		assert.EqualValues(t, 1, fetcher.calls.Load())
		require.Error(t, evalExpr(t, env, context.Background(), expr, bind(context.Background())))
		assert.EqualValues(t, 2, fetcher.calls.Load())
	})
	t.Run("deadline reaches the fetch through the activation", func(t *testing.T) {
		reg.delay.Store(int64(5 * time.Second))
		defer reg.delay.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		err := evalExpr(t, env, ctx, fourCalls, bind(ctx))
		require.Error(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
	})
}

func TestRequestContext_DifferentRemoteOptionsAreNotShared(t *testing.T) {
	reg := newTestRegistry(t, "a:v1")
	provider, fetcher := newTestProvider(t)
	rc := WithRequestContext(context.Background(), provider)
	image := reg.host + "/a:v1"
	optsA := []remote.Option{remote.WithUserAgent("a")}
	optsB := []remote.Option{remote.WithUserAgent("b")}
	for _, opts := range [][]remote.Option{optsA, optsA, optsB, optsB, nil, nil} {
		_, err := rc.GetImageData(image, opts)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 3, fetcher.calls.Load())
}

func TestRequestContext_CloneDoesNotShareMemo(t *testing.T) {
	reg := newTestRegistry(t, "a:v1")
	provider, fetcher := newTestProvider(t)
	rc := WithRequestContext(context.Background(), provider)
	image := reg.host + "/a:v1"
	_, err := rc.GetImageData(image, nil)
	require.NoError(t, err)
	_, err = rc.Clone().GetImageData(image, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, fetcher.calls.Load())
}

func TestRequestContext_ConcurrentEvaluationsKeepTheirOwnContext(t *testing.T) {
	reg := newTestRegistry(t, "a:v1")
	provider, fetcher := newTestProvider(t)
	env := newImageEnv(t, provider, nil)
	image := reg.host + "/a:v1"
	expr := `image.GetMetadata("` + image + `").manifest != null && image.GetMetadata("` + image + `").manifest != null`
	reg.delay.Store(int64(200 * time.Millisecond))

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if i%2 == 1 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer cancel()
			}
			activation := map[string]any{}
			BindRequestContext(ctx, provider, activation)
			errs[i] = evalExpr(t, env, ctx, expr, activation)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if i%2 == 1 {
			assert.Error(t, err, "worker %d has a short deadline", i)
		} else {
			assert.NoError(t, err, "worker %d has no deadline", i)
		}
	}
	assert.EqualValues(t, workers, fetcher.calls.Load(), "one fetch per evaluation")
}

// ctxRecorder is a dynamic client that records the context it receives.
type ctxRecorder struct {
	dynamic.Interface
	dynamic.ResourceInterface
	getCtx  context.Context
	listCtx context.Context
}

type ctxKey struct{}

func (c *ctxRecorder) Resource(schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return c
}

func (c *ctxRecorder) Namespace(string) dynamic.ResourceInterface { return c }

func (c *ctxRecorder) Get(ctx context.Context, name string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	c.getCtx = ctx
	return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": name}}}, ctx.Err()
}

func (c *ctxRecorder) List(ctx context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	c.listCtx = ctx
	return &unstructured.UnstructuredList{}, ctx.Err()
}

type fakeDClient struct {
	dclient.Interface
	dyn dynamic.Interface
}

func (f *fakeDClient) GetDynamicInterface() dynamic.Interface { return f.dyn }

func TestRequestContext_ResourceCallsUseEvaluationContext(t *testing.T) {
	recorder := &ctxRecorder{}
	provider := &contextProvider{client: &fakeDClient{dyn: recorder}}
	env, err := cel.NewEnv(resource.Lib(resource.Context{ContextInterface: provider}, "", resource.Latest()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "request"))
	activation := map[string]any{}
	BindRequestContext(ctx, provider, activation)

	require.NoError(t, evalExpr(t, env, ctx, `resource.Get("v1", "configmaps", "ns", "cm").metadata.name == "cm"`, activation))
	require.NoError(t, evalExpr(t, env, ctx, `size(resource.List("v1", "configmaps", "ns")) == 0`, activation))
	require.NotNil(t, recorder.getCtx)
	require.NotNil(t, recorder.listCtx)
	assert.Equal(t, "request", recorder.getCtx.Value(ctxKey{}))
	assert.Equal(t, "request", recorder.listCtx.Value(ctxKey{}))

	cancel()
	assert.Error(t, evalExpr(t, env, ctx, `resource.Get("v1", "configmaps", "ns", "cm").metadata.name == "cm"`, activation))
}

func TestBindRequestContext_NilIsNoop(t *testing.T) {
	activation := map[string]any{}
	BindRequestContext(context.Background(), nil, activation)
	assert.Empty(t, activation)
	assert.Nil(t, WithRequestContext(context.Background(), nil))
}
