package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/cache"
)

type mockToggles struct{}

func (mockToggles) ProtectManagedResources() bool           { return false }
func (mockToggles) ForceFailurePolicyIgnore() bool          { return false }
func (mockToggles) EnableDeferredLoading() bool             { return false }
func (mockToggles) GenerateValidatingAdmissionPolicy() bool { return false }
func (mockToggles) GenerateMutatingAdmissionPolicy() bool   { return false }
func (mockToggles) DumpMutatePatches() bool                 { return false }
func (mockToggles) AutogenV2() bool                         { return false }
func (mockToggles) AllowHTTPInNamespacedPolicies() bool     { return false }

func TestReconcile_Coverage(t *testing.T) {
	c := &controller{
		cpolLister: kyvernov1listers.NewClusterPolicyLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		vpolLister: policiesv1beta1listers.NewValidatingPolicyLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
		mpolLister: policiesv1beta1listers.NewMutatingPolicyLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	ctx := toggle.NewContext(context.Background(), mockToggles{})

	err := c.reconcile(ctx, logr.Discard(), "ClusterPolicy/test", "", "test")
	assert.NoError(t, err)

	err = c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/test", "", "test")
	assert.NoError(t, err)

	err = c.reconcile(ctx, logr.Discard(), "MutatingPolicy/test", "", "test")
	assert.NoError(t, err)
}
