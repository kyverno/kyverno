package cleanup

import (
	"context"
	"io"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cleanupContextClient struct {
	dclient.Interface
	paths []string
}

func (c *cleanupContextClient) RawAbsPath(_ context.Context, path, _ string, _ io.Reader) ([]byte, error) {
	c.paths = append(c.paths, path)
	return []byte(`{"items":[]}`), nil
}

type cleanupContextEntry struct {
	calls int
}

func (e *cleanupContextEntry) Get(string) (any, error) {
	e.calls++
	return []any{}, nil
}

func (*cleanupContextEntry) Stop() {}

type eagerCleanupContext struct{ toggle.Toggles }

func (eagerCleanupContext) EnableDeferredLoading() bool { return false }

func TestCleanupContextPreservesClusterScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		entry kyvernov1.ContextEntry
		path  string
	}{
		{
			name: "cross namespace API call",
			entry: kyvernov1.ContextEntry{Name: "objects", APICall: &kyvernov1.ContextAPICall{
				APICall: kyvernov1.APICall{URLPath: "/api/v1/namespaces/shared/configmaps"},
			}},
			path: "/api/v1/namespaces/shared/configmaps",
		},
		{
			name: "cluster API call",
			entry: kyvernov1.ContextEntry{Name: "objects", APICall: &kyvernov1.ContextAPICall{
				APICall: kyvernov1.APICall{URLPath: "/api/v1/namespaces"},
			}},
			path: "/api/v1/namespaces",
		},
		{
			name:  "global reference",
			entry: kyvernov1.ContextEntry{Name: "objects", GlobalReference: &kyvernov1.GlobalContextEntryReference{Name: "shared-context"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, namespace := range []string{"tenant-a", ""} {
				t.Run("namespace="+namespace, func(t *testing.T) {
					t.Parallel()
					client := &cleanupContextClient{}
					entry := &cleanupContextEntry{}
					gctx := store.New(0)
					require.NoError(t, gctx.Set("shared-context", entry))
					spec := kyvernov2.CleanupPolicySpec{Context: []kyvernov1.ContextEntry{test.entry}}
					var policy kyvernov2.CleanupPolicyInterface = &kyvernov2.ClusterCleanupPolicy{Spec: spec}
					if namespace != "" {
						policy = &kyvernov2.CleanupPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}, Spec: spec}
					}
					c := &controller{client: client, gctxStore: gctx, jp: jmespath.New(config.NewDefaultConfiguration(false))}
					// Force context evaluation even though there are no cleanup
					// candidates; this checks the controller's actual scope choice.
					ctx := context.Background()
					ctx = toggle.NewContext(ctx, eagerCleanupContext{toggle.FromContext(ctx)})
					require.NoError(t, c.cleanup(ctx, logr.Discard(), policy))
					if test.path != "" {
						require.Equal(t, []string{test.path}, client.paths)
					} else {
						require.Equal(t, 1, entry.calls)
					}
				})
			}
		})
	}
}
