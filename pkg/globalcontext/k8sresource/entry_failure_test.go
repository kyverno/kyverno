package k8sresource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// newMustReturn asserts that New returns an error instead of hanging when the
// informer's list/watch permanently fails. Regression test for #15856: the
// watch error handler used to call stop() (which does group.Wait()) from the
// reflector's own goroutine, self-deadlocking and permanently blocking the
// globalcontext controller's single reconcile worker.
func newMustReturn(t *testing.T, ctx context.Context, host string) {
	t.Helper()
	dyn, err := dynamic.NewForConfig(&rest.Config{Host: host})
	require.NoError(t, err)
	gce := &kyvernov2beta1.GlobalContextEntry{}
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := New(ctx, gce, &mockEventGen{}, dyn, logging.GlobalLogger(), gvr, "", nil)
		done <- result{err}
	}()

	select {
	case r := <-done:
		assert.Error(t, r.err)
	case <-time.After(30 * time.Second):
		t.Fatal("New did not return after permanent informer failure; the reconcile worker would deadlock (#15856)")
	}
}

func TestNew_MissingCRD(t *testing.T) {
	// A healthy API server that returns 404 for the referenced resource,
	// as happens when a GlobalContextEntry references a non-existent CRD.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","message":"the server could not find the requested resource","code":404}`))
	}))
	defer srv.Close()
	newMustReturn(t, context.Background(), srv.URL)
}

func TestNew_UnreachableServer(t *testing.T) {
	// client-go retries connection-refused errors internally before the watch
	// error handler fires, so this path relies on the bounded cache sync. Use
	// a short context deadline to keep the test fast; New derives its sync
	// bound from the caller's context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	newMustReturn(t, ctx, "https://127.0.0.1:1")
}
