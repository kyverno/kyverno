package libs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type imageRequestKey struct{}

type imageContextTransport struct {
	manifest, config []byte
	started          chan context.Context
	abort            <-chan struct{}
}

func (tr *imageContextTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Context().Value(imageRequestKey{}) == "cancel" {
		tr.started <- request.Context()
		select {
		case <-request.Context().Done():
			return nil, request.Context().Err()
		case <-tr.abort:
			return nil, errors.New("test registry stopped")
		}
	}
	data, contentType := []byte("{}"), "application/json"
	if strings.Contains(request.URL.Path, "/manifests/") {
		data, contentType = tr.manifest, "application/vnd.docker.distribution.manifest.v2+json"
	} else if strings.Contains(request.URL.Path, "/blobs/") {
		data = tr.config
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data)), Request: request}, nil
}

func TestImageDataContextCancelsRegistryRequestsAndKeepsConcurrentEvaluationsIndependent(t *testing.T) {
	t.Parallel()
	manifest, err := empty.Image.RawManifest()
	require.NoError(t, err)
	config, err := empty.Image.RawConfigFile()
	require.NoError(t, err)
	abort := make(chan struct{})
	defer close(abort)
	transport := &imageContextTransport{manifest: manifest, config: config, started: make(chan context.Context, 1), abort: abort}
	fetcher, err := imagedataloader.New(nil, []remote.Option{remote.WithTransport(transport)}, nil)
	require.NoError(t, err)
	provider := &contextProvider{imagedata: fetcher}
	env, err := cel.NewEnv(imagedata.Lib(imagedata.Context{ContextInterface: provider}, imagedata.Latest(), nil))
	require.NoError(t, err)
	ast, issues := env.Compile(`image.GetMetadata("registry.example/fixture:latest").resolvedImage`)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), imageRequestKey{}, "cancel"), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, _, err := program.ContextEval(ctx, map[string]any{"image": ImageDataContext(ctx, provider)})
		finished <- err
	}()
	select {
	case requestCtx := <-transport.started:
		deadline, ok := requestCtx.Deadline()
		require.True(t, ok)
		expectedDeadline, _ := ctx.Deadline()
		assert.Equal(t, expectedDeadline, deadline)
	case <-time.After(time.Second):
		t.Fatal("CEL metadata lookup did not receive its evaluation context")
	}
	// A shared compiled program and provider must still serve other requests
	// while the first lookup is blocked, and after that request is cancelled.
	var calls sync.WaitGroup
	for range 4 {
		calls.Go(func() {
			successCtx := context.Background()
			result, _, err := program.ContextEval(successCtx, map[string]any{"image": ImageDataContext(successCtx, provider)})
			assert.NoError(t, err)
			if err == nil {
				assert.Contains(t, result.Value(), "registry.example/fixture:latest@sha256:")
			}
		})
	}
	calls.Wait()
	cancel()
	select {
	case err := <-finished:
		require.ErrorContains(t, err, "context canceled")
	case <-time.After(time.Second):
		t.Fatal("CEL metadata lookup ignored evaluation cancellation")
	}
	result, _, err := program.ContextEval(context.Background(), map[string]any{"image": ImageDataContext(context.Background(), provider)})
	require.NoError(t, err)
	assert.Contains(t, result.Value(), "registry.example/fixture:latest@sha256:")
}

func TestImageDataContextPreservesFixtureProviders(t *testing.T) {
	t.Parallel()
	provider := &imagedata.ContextMock{GetImageDataFunc: func(image string, _ []remote.Option) (map[string]any, error) {
		return map[string]any{"resolvedImage": image}, nil
	}}
	result, err := ImageDataContext(context.Background(), provider).GetImageData("fixture", nil)
	require.NoError(t, err)
	assert.Equal(t, "fixture", result["resolvedImage"])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ImageDataContext(ctx, provider).GetImageData("fixture", nil)
	require.ErrorIs(t, err, context.Canceled)
}
