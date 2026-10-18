package dclient

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	restclient "k8s.io/client-go/rest"
)

// flippingDiscovery is a real AggregatedDiscoveryInterface whose resource
// list flips at readyAt; other methods panic, unused by this refresh path.
type flippingDiscovery struct {
	gv       schema.GroupVersion
	kind     string
	resource string
	readyAt  time.Time
	calls    atomic.Int32
}

func (f *flippingDiscovery) GroupsAndMaybeResources() (*metav1.APIGroupList, map[schema.GroupVersion]*metav1.APIResourceList, map[schema.GroupVersion]error, error) {
	f.calls.Add(1)
	groups := &metav1.APIGroupList{}
	resources := map[schema.GroupVersion]*metav1.APIResourceList{}
	if !time.Now().Before(f.readyAt) {
		groups.Groups = append(groups.Groups, metav1.APIGroup{
			Name: f.gv.Group,
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: f.gv.String(), Version: f.gv.Version},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: f.gv.String(), Version: f.gv.Version},
		})
		resources[f.gv] = &metav1.APIResourceList{
			GroupVersion: f.gv.String(),
			APIResources: []metav1.APIResource{
				{Name: f.resource, Kind: f.kind, Group: f.gv.Group, Version: f.gv.Version, Namespaced: true},
			},
		}
	}
	return groups, resources, nil, nil
}

func (f *flippingDiscovery) RESTClient() restclient.Interface { return nil }
func (f *flippingDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) ServerVersion() (*version.Info, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) OpenAPIV3() openapi.Client {
	panic("not used by the aggregated refresh path")
}

func (f *flippingDiscovery) WithLegacy() discovery.DiscoveryInterface {
	panic("not used by the aggregated refresh path")
}

var _ discovery.AggregatedDiscoveryInterface = &flippingDiscovery{}

func newStaleCacheDiscovery(t *testing.T, readyAfter time.Duration) (IDiscovery, *flippingDiscovery) {
	t.Helper()
	fake := &flippingDiscovery{
		gv:       schema.GroupVersion{Group: "cilium.io", Version: "v2"},
		kind:     "CiliumNetworkPolicy",
		resource: "ciliumnetworkpolicies",
		readyAt:  time.Now().Add(readyAfter),
	}
	disco := NewServerResourcesDiscovery(fake)
	// Fill the cache before the CRD exists: a valid snapshot without it.
	_, err := disco.CachedDiscoveryInterface().ServerGroups()
	require.NoError(t, err)
	require.True(t, disco.CachedDiscoveryInterface().Fresh())
	return disco, fake
}

// TestDiscoveryMiss_RefetchesValidCache: a valid snapshot taken before the
// CRD existed must not hide it forever; a later miss re-asks the server.
func TestDiscoveryMiss_RefetchesValidCache(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	gvr := gvk.GroupVersion().WithResource("ciliumnetworkpolicies")
	lookups := map[string]func(*testing.T, IDiscovery){
		"GetGVRFromGVK": func(t *testing.T, d IDiscovery) {
			got, err := d.GetGVRFromGVK(gvk)
			require.NoError(t, err)
			assert.Equal(t, gvr, got)
		},
		"GetGVKFromGVR": func(t *testing.T, d IDiscovery) {
			got, err := d.GetGVKFromGVR(gvr)
			require.NoError(t, err)
			assert.Equal(t, gvk, got)
		},
		"FindResource": func(t *testing.T, d IDiscovery) {
			_, _, got, err := d.(*serverResources).FindResource(gvk.GroupVersion().String(), gvk.Kind)
			require.NoError(t, err)
			assert.Equal(t, gvr, got)
		},
	}
	for name, lookup := range lookups {
		t.Run(name, func(t *testing.T) {
			disco, fake := newStaleCacheDiscovery(t, 50*time.Millisecond)
			time.Sleep(100 * time.Millisecond)
			filled := fake.calls.Load()

			lookup(t, disco)

			assert.Greater(t, fake.calls.Load(), filled, "the miss must have re-asked the server")
		})
	}
}

// TestDiscoveryMiss_RefetchIsRateLimited: a kind that never appears (a
// policy with a typo, a CRD that was never installed) must not turn every
// lookup into a discovery refetch.
func TestDiscoveryMiss_RefetchIsRateLimited(t *testing.T) {
	disco, fake := newStaleCacheDiscovery(t, time.Hour)
	filled := fake.calls.Load()
	missing := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}

	start := time.Now()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				_, err := disco.GetGVRFromGVK(missing)
				assert.Error(t, err)
			}
		})
	}
	wg.Wait()

	refetches := int(fake.calls.Load() - filled)
	// One refetch is allowed per interval, plus the first one.
	assert.GreaterOrEqual(t, refetches, 1, "a miss against a valid cache must refetch at least once")
	assert.LessOrEqual(t, refetches, 1+int(time.Since(start)/missRefetchInterval), "800 concurrent misses must not each refetch")
}
