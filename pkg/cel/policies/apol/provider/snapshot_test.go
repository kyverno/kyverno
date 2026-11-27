package provider

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSnapshotLifecycle(t *testing.T) {
	t.Parallel()
	r := &reconciler{compiler: apolcompiler.NewCompiler(), policies: map[string]apolengine.CompiledPolicy{}}
	_, err := r.Snapshot(context.Background())
	require.ErrorIs(t, err, apolengine.ErrNoSnapshot, "no snapshot before warm-up")

	pol := &policiesv1alpha1.AuthorizingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p"}}
	r.lock.Lock()
	r.policies["p"] = apolengine.NewCompiledPolicy(r.compiler, pol)
	r.publish()
	r.lock.Unlock()
	_, err = r.Snapshot(context.Background())
	require.ErrorIs(t, err, apolengine.ErrNoSnapshot, "reconciles before warm-up do not publish")

	r.lock.Lock()
	r.warm = true
	r.publish()
	r.lock.Unlock()
	first, err := r.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, first.Len())

	r.lock.Lock()
	delete(r.policies, "p")
	r.publish()
	r.lock.Unlock()
	second, err := r.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, second.Len())
	require.Greater(t, second.Generation(), first.Generation())
	require.Equal(t, 1, first.Len(), "published snapshots are immutable")
}
