package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestWarmUp(t *testing.T) {
	keys := []client.ObjectKey{{Name: "a"}, {Name: "broken"}, {Namespace: "ns", Name: "b"}}

	t.Run("reconciles every key and skips failures", func(t *testing.T) {
		var reconciled []string
		r := reconcile.Func(func(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
			reconciled = append(reconciled, req.NamespacedName.String())
			if req.Name == "broken" {
				return reconcile.Result{}, errors.New("compile error")
			}
			return reconcile.Result{}, nil
		})
		require.NoError(t, WarmUp(context.Background(), "test", r, keys))
		assert.Equal(t, []string{"/a", "/broken", "ns/b"}, reconciled)
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var reconciled int
		r := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
			reconciled++
			cancel()
			return reconcile.Result{}, nil
		})
		require.ErrorIs(t, WarmUp(ctx, "test", r, keys), context.Canceled)
		assert.Equal(t, 1, reconciled)
	})
}
