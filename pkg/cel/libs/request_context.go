package libs

import (
	"context"
	"sync"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// contextAware is implemented by Context implementations that can honor a caller supplied
// context.Context for the calls that reach out to the registry or the API server.
type contextAware interface {
	GetImageDataContext(ctx context.Context, image string, remoteOpts []remote.Option) (map[string]any, error)
	GetResourceContext(ctx context.Context, apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
	ListResourcesContext(ctx context.Context, apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error)
}

type imageKey struct {
	image string
	nopts int
	// the address of the first option identifies the option slice; slices built separately
	// never share an entry, so differing credentials are never mixed up.
	opts *remote.Option
}

type imageResult struct {
	once sync.Once
	data map[string]any
	err  error
}

type requestContext struct {
	Context
	ctx context.Context

	mu     sync.Mutex
	images map[imageKey]*imageResult
}

// WithRequestContext returns a Context scoped to a single policy evaluation. The CEL library
// functions bound at compile time cannot see the evaluation context, so the returned value is
// meant to be placed in the evaluation activation (under the "image" and "resource" keys), where it
// takes precedence over the compile-time globals. It:
//   - uses ctx for registry and API server calls, so webhook deadlines and cancellation apply;
//   - memoizes GetImageData (results and errors) for the lifetime of the wrapper only, so repeated
//     image.GetMetadata calls for the same image within one evaluation fetch once.
//
// The memo is never shared across evaluations or Clone()s, and everything else is delegated to c.
// Create one wrapper per evaluation; do not retain it.
func WithRequestContext(ctx context.Context, c Context) Context {
	if c == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if rc, ok := c.(*requestContext); ok {
		c = rc.Context
	}
	return &requestContext{
		Context: c,
		ctx:     ctx,
		images:  map[imageKey]*imageResult{},
	}
}

func (rc *requestContext) GetImageData(image string, remoteOpts []remote.Option) (map[string]any, error) {
	key := imageKey{image: image, nopts: len(remoteOpts)}
	if len(remoteOpts) > 0 {
		key.opts = &remoteOpts[0]
	}
	rc.mu.Lock()
	entry, ok := rc.images[key]
	if !ok {
		entry = &imageResult{}
		rc.images[key] = entry
	}
	rc.mu.Unlock()
	entry.once.Do(func() {
		if ca, ok := rc.Context.(contextAware); ok {
			entry.data, entry.err = ca.GetImageDataContext(rc.ctx, image, remoteOpts)
		} else {
			entry.data, entry.err = rc.Context.GetImageData(image, remoteOpts)
		}
	})
	return entry.data, entry.err
}

func (rc *requestContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	if ca, ok := rc.Context.(contextAware); ok {
		return ca.GetResourceContext(rc.ctx, apiVersion, resource, namespace, name)
	}
	return rc.Context.GetResource(apiVersion, resource, namespace, name)
}

func (rc *requestContext) ListResources(apiVersion, resource, namespace string, l map[string]string) (*unstructured.UnstructuredList, error) {
	if ca, ok := rc.Context.(contextAware); ok {
		return ca.ListResourcesContext(rc.ctx, apiVersion, resource, namespace, l)
	}
	return rc.Context.ListResources(apiVersion, resource, namespace, l)
}

// Clone returns a fresh request scoped wrapper over a clone of the underlying context, so the
// memo is not shared.
func (rc *requestContext) Clone() Context {
	return WithRequestContext(rc.ctx, rc.Context.Clone())
}

// BindRequestContext binds a request scoped view of c (see WithRequestContext) to the "image" and
// "resource" variables of an evaluation activation, so the imagedata and resource libraries use
// ctx and share one image metadata memo for this evaluation. It is a no-op when c is nil.
// Call it once per evaluation, with the activation map every program of that evaluation uses.
func BindRequestContext(ctx context.Context, c Context, activation map[string]any) {
	if c == nil || activation == nil {
		return
	}
	rc := WithRequestContext(ctx, c)
	// keys mirror the cel.Variable declarations of the imagedata and resource libraries
	activation["image"] = imagedata.Context{ContextInterface: rc}
	activation["resource"] = resource.Context{ContextInterface: rc}
}
