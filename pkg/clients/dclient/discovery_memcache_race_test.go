package dclient

import (
	"fmt"
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
		"FindResources": func(t *testing.T, d IDiscovery) {
			got, err := d.FindResources(gvk.Group, gvk.Version, gvk.Kind, "")
			require.NoError(t, err)
			require.Len(t, got, 1)
			for resource := range got {
				assert.Equal(t, gvr, resource.GroupVersionResource())
			}
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
	missing := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	lookups := map[string]func(IDiscovery) error{
		"GetGVRFromGVK": func(d IDiscovery) error {
			_, err := d.GetGVRFromGVK(missing)
			return err
		},
		"FindResources": func(d IDiscovery) error {
			_, err := d.FindResources(missing.Group, missing.Version, missing.Kind, "")
			return err
		},
	}
	for name, lookup := range lookups {
		t.Run(name, func(t *testing.T) {
			disco, fake := newStaleCacheDiscovery(t, time.Hour)
			filled := fake.calls.Load()
			start := time.Now()
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					for range 100 {
						assert.Error(t, lookup(disco))
					}
				})
			}
			wg.Wait()

			refetches := int(fake.calls.Load() - filled)
			// One refetch is allowed per interval, plus the first one.
			assert.GreaterOrEqual(t, refetches, 1, "a miss against a valid cache must refetch at least once")
			assert.LessOrEqual(t, refetches, 1+int(time.Since(start)/missRefetchInterval), "800 concurrent misses must not each refetch")
		})
	}
}

func TestDiscoveryMiss_StaleCacheRefetchIsShared(t *testing.T) {
	missing := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	resource := missing.GroupVersion().WithResource("ciliumnetworkpolicies")
	lookups := map[string]func(*serverResources) error{
		"GetGVRFromGVK": func(d *serverResources) error {
			_, err := d.GetGVRFromGVK(missing)
			return err
		},
		"GetGVKFromGVR": func(d *serverResources) error {
			_, err := d.GetGVKFromGVR(resource)
			return err
		},
		"FindResource": func(d *serverResources) error {
			_, _, _, err := d.FindResource(missing.GroupVersion().String(), missing.Kind)
			return err
		},
		"FindResources": func(d *serverResources) error {
			_, err := d.FindResources(missing.Group, missing.Version, missing.Kind, "")
			return err
		},
	}
	for name, lookup := range lookups {
		t.Run(name, func(t *testing.T) {
			for _, recentRefetch := range []bool{true, false} {
				t.Run(fmt.Sprintf("recentRefetch=%t", recentRefetch), func(t *testing.T) {
					newClient := func() (*serverResources, *flippingDiscovery, time.Time) {
						disco, fake := newStaleCacheDiscovery(t, time.Hour)
						d := disco.(*serverResources)
						refetchStart := time.Now()
						if recentRefetch {
							_, err := d.GetGVRFromGVK(missing)
							require.Error(t, err)
							require.WithinDuration(t, time.Now(), d.lastRefetch, missRefetchInterval)
						}
						// Simulate Poll invalidating the discovery cache and mapper.
						d.cachedClient.Invalidate()
						d.mapper.Reset()
						return d, fake, refetchStart
					}
					single, singleFake, _ := newClient()
					filled := singleFake.calls.Load()
					require.Error(t, lookup(single))
					want := singleFake.calls.Load() - filled
					if recentRefetch {
						require.EqualValues(t, 1, want, "a stale cache must refresh even inside the miss interval")
					}

					d, fake, refetchStart := newClient()
					filled = fake.calls.Load()
					start := make(chan struct{})
					var wg sync.WaitGroup
					for range 8 {
						wg.Go(func() {
							<-start
							for range 100 {
								assert.Error(t, lookup(d))
							}
						})
					}
					if !recentRefetch {
						refetchStart = time.Now()
					}
					close(start)
					wg.Wait()

					calls := fake.calls.Load() - filled
					assert.GreaterOrEqual(t, int(calls), int(want), "concurrent misses must share the stale cache refresh")
					assert.LessOrEqual(t, int(calls), int(want)+int(time.Since(refetchStart)/missRefetchInterval), "concurrent misses must share the stale cache refresh")
				})
			}
		})
	}
}

func TestFindResources_InvalidationFindsNewResource(t *testing.T) {
	disco, fake := newStaleCacheDiscovery(t, time.Hour)
	resources, err := disco.FindResources(fake.gv.Group, fake.gv.Version, fake.kind, "")
	require.ErrorIs(t, err, ErrResourceNotFound)
	require.Nil(t, resources)
	d := disco.(*serverResources)
	lastRefetch := d.lastRefetch
	filled := fake.calls.Load()

	// A CRD event invalidates discovery even inside the miss refetch interval.
	fake.readyAt = time.Now()
	d.cachedClient.Invalidate()
	d.mapper.Reset()
	resources, err = disco.FindResources(fake.gv.Group, fake.gv.Version, fake.kind, "")
	require.NoError(t, err)
	require.Len(t, resources, 1)
	for resource := range resources {
		assert.Equal(t, fake.gv.WithResource(fake.resource), resource.GroupVersionResource())
	}
	assert.EqualValues(t, 1, fake.calls.Load()-filled, "a stale cache must refresh immediately")
	assert.Equal(t, lastRefetch, d.lastRefetch, "a cache refresh must not need a forced miss refetch")
}
