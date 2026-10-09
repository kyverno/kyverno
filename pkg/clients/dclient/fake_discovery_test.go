package dclient

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

func TestFakeDiscoveryFindResources(t *testing.T) {
	t.Parallel()
	alphaV1 := schema.GroupVersionResource{Group: "alpha.example", Version: "v1", Resource: "widgets"}
	alphaV2 := schema.GroupVersionResource{Group: "alpha.example", Version: "v2", Resource: "widgets"}
	beta := schema.GroupVersionResource{Group: "beta.example", Version: "v1", Resource: "widgets"}
	core := schema.GroupVersionResource{Version: "v1", Resource: "widgets"}
	irregular := schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	for _, test := range []struct {
		name, group, version, kind string
		want                       map[schema.GroupVersionResource]bool
	}{
		{name: "exact group and version", group: "alpha.example", version: "v1", kind: "Widget", want: map[schema.GroupVersionResource]bool{alphaV1: true}},
		{name: "all versions preserve scopes", group: "alpha.example", version: "*", kind: "Widget", want: map[schema.GroupVersionResource]bool{alphaV1: true, alphaV2: false}},
		{name: "all groups", group: "*", version: "v1", kind: "Widget", want: map[schema.GroupVersionResource]bool{alphaV1: true, beta: false, core: true}},
		{name: "all groups and versions", group: "*", version: "*", kind: "Widget", want: map[schema.GroupVersionResource]bool{alphaV1: true, alphaV2: false, beta: false, core: true}},
		{name: "wildcard patterns", group: "*.example", version: "v?", kind: "Wid*", want: map[schema.GroupVersionResource]bool{alphaV1: true, alphaV2: false, beta: false}},
		{name: "empty group is core", version: "v1", kind: "Widget", want: map[schema.GroupVersionResource]bool{core: true}},
		{name: "empty version is not wildcard", group: "alpha.example", kind: "Widget"},
		{name: "unknown group", group: "missing.example", version: "v1", kind: "Widget"},
		{name: "unknown version", group: "alpha.example", version: "v9", kind: "Widget"},
		{name: "irregular plural", group: "networking.k8s.io", version: "v1", kind: "NetworkPolicy", want: map[schema.GroupVersionResource]bool{irregular: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			discovery := NewFakeDiscoveryClient([]schema.GroupVersionResource{beta, alphaV2, alphaV1, core, irregular, alphaV1})
			for gvr, namespaced := range map[schema.GroupVersionResource]bool{alphaV1: true, alphaV2: false, beta: false, core: true, irregular: true} {
				kind := "Widget"
				if gvr == irregular {
					kind = "NetworkPolicy"
				}
				discovery.AddGVRToGVKMapping(gvr, gvr.GroupVersion().WithKind(kind))
				discovery.SetResourceScope(gvr, namespaced)
			}
			resources, err := discovery.FindResources(test.group, test.version, test.kind, "")
			if len(test.want) == 0 {
				require.Error(t, err)
				require.Empty(t, resources)
				return
			}
			require.NoError(t, err)
			got := make(map[schema.GroupVersionResource]bool)
			for description, resource := range resources {
				got[description.GroupVersionResource()] = resource.Namespaced
				require.NotContains(t, description.Kind, "*")
			}
			require.Equal(t, test.want, got)
		})
	}
}

func TestFakeDiscoveryGetGVRFromGVK(t *testing.T) {
	t.Parallel()
	alphaV1 := schema.GroupVersionResource{Group: "alpha.example", Version: "v1", Resource: "widgets"}
	alphaV2 := schema.GroupVersionResource{Group: "alpha.example", Version: "v2", Resource: "widgets"}
	beta := schema.GroupVersionResource{Group: "beta.example", Version: "v1", Resource: "widgets"}
	core := schema.GroupVersionResource{Version: "v1", Resource: "widgets"}
	irregular := schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	for _, mapped := range []bool{false, true} {
		for _, test := range []struct {
			name      string
			gvk       schema.GroupVersionKind
			want      schema.GroupVersionResource
			wantError bool
		}{
			{name: "exact version", gvk: alphaV1.GroupVersion().WithKind("Widget"), want: alphaV1},
			{name: "exact core group", gvk: core.GroupVersion().WithKind("Widget"), want: core},
			{name: "missing version", gvk: schema.GroupVersionKind{Group: "alpha.example", Version: "v9", Kind: "Widget"}, wantError: true},
			{name: "missing group", gvk: schema.GroupVersionKind{Group: "missing.example", Version: "v1", Kind: "Widget"}, wantError: true},
		} {
			name := "inferred/" + test.name
			if mapped {
				name = "mapped/" + test.name
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				discovery := NewFakeDiscoveryClient([]schema.GroupVersionResource{beta, alphaV2, alphaV1, core})
				if mapped {
					for _, gvr := range []schema.GroupVersionResource{beta, alphaV2, alphaV1, core} {
						discovery.AddGVRToGVKMapping(gvr, gvr.GroupVersion().WithKind("Widget"))
					}
				}
				got, err := discovery.GetGVRFromGVK(test.gvk)
				if test.wantError {
					require.Error(t, err)
					require.Empty(t, got)
					return
				}
				require.NoError(t, err)
				require.Equal(t, test.want, got)
			})
		}
	}
	t.Run("omitted irregular version", func(t *testing.T) {
		t.Parallel()
		discovery := NewFakeDiscoveryClient([]schema.GroupVersionResource{irregular})
		discovery.AddGVRToGVKMapping(irregular, irregular.GroupVersion().WithKind("NetworkPolicy"))
		got, err := discovery.GetGVRFromGVK(schema.GroupVersionKind{Kind: "NetworkPolicy"})
		require.NoError(t, err)
		require.Equal(t, irregular, got)
	})
}

func TestFakeDiscoveryPreferredVersion(t *testing.T) {
	t.Parallel()
	v1 := schema.GroupVersionResource{Group: "networking.example", Version: "v1", Resource: "networkpolicies"}
	v2 := schema.GroupVersionResource{Group: "networking.example", Version: "v2", Resource: "networkpolicies"}
	other := schema.GroupVersionResource{Group: "other.example", Version: "v2", Resource: "networkpolicies"}
	for _, test := range []struct {
		name       string
		registered []schema.GroupVersionResource
		preferred  string
		gvk        schema.GroupVersionKind
		want       schema.GroupVersionResource
		wantError  bool
	}{
		{name: "preferred version", registered: []schema.GroupVersionResource{v1, v2}, preferred: "v2", gvk: schema.GroupVersionKind{Kind: "NetworkPolicy"}, want: v2},
		{name: "preference independent of order", registered: []schema.GroupVersionResource{v2, v1}, preferred: "v2", gvk: schema.GroupVersionKind{Kind: "NetworkPolicy"}, want: v2},
		{name: "explicit version overrides preference", registered: []schema.GroupVersionResource{v2, v1}, preferred: "v2", gvk: v1.GroupVersion().WithKind("NetworkPolicy"), want: v1},
		{name: "group with omitted version", registered: []schema.GroupVersionResource{v1, v2}, preferred: "v2", gvk: schema.GroupVersionKind{Group: v1.Group, Kind: "NetworkPolicy"}, want: v2},
		{name: "ambiguous versions", registered: []schema.GroupVersionResource{v1, v2}, gvk: schema.GroupVersionKind{Kind: "NetworkPolicy"}, wantError: true},
		{name: "ambiguous groups", registered: []schema.GroupVersionResource{other, v1, v2}, preferred: "v2", gvk: schema.GroupVersionKind{Kind: "NetworkPolicy"}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			discovery := NewFakeDiscoveryClient(test.registered)
			for _, gvr := range test.registered {
				discovery.AddGVRToGVKMapping(gvr, gvr.GroupVersion().WithKind("NetworkPolicy"))
			}
			discovery.SetPreferredVersion(v1.Group, test.preferred)
			got, err := discovery.GetGVRFromGVK(test.gvk)
			if test.wantError {
				require.ErrorContains(t, err, "ambiguous")
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestFakeDiscoveryCorePreference(t *testing.T) {
	t.Parallel()
	core := schema.GroupVersionResource{Version: "v1", Resource: "events"}
	events := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
	groups := []*restmapper.APIGroupResources{}
	discovery := NewFakeDiscoveryClient([]schema.GroupVersionResource{events, core})
	for _, gvr := range []schema.GroupVersionResource{events, core} {
		discovery.AddGVRToGVKMapping(gvr, gvr.GroupVersion().WithKind("Event"))
		discovery.SetPreferredVersion(gvr.Group, gvr.Version)
		discovery.SetResourceScope(gvr, true)
		version := metav1.GroupVersionForDiscovery{GroupVersion: gvr.GroupVersion().String(), Version: gvr.Version}
		groups = append(groups, &restmapper.APIGroupResources{
			Group: metav1.APIGroup{Name: gvr.Group, Versions: []metav1.GroupVersionForDiscovery{version}, PreferredVersion: version},
			VersionedResources: map[string][]metav1.APIResource{
				gvr.Version: {{Name: "events", Kind: "Event", Namespaced: true}},
			},
		})
	}
	// Match the runtime RESTMapper's core/v1 preference, regardless of group order.
	mapping, err := restmapper.NewDiscoveryRESTMapper(groups).RESTMapping(schema.GroupKind{Kind: "Event"})
	require.NoError(t, err)
	require.Equal(t, core, mapping.Resource)
	got, err := discovery.GetGVRFromGVK(schema.GroupVersionKind{Kind: "Event"})
	require.NoError(t, err)
	require.Equal(t, mapping.Resource, got)
	got, err = discovery.GetGVRFromGVK(events.GroupVersion().WithKind("Event"))
	require.NoError(t, err)
	require.Equal(t, events, got, "an explicit group must override core preference")
	resources, err := discovery.FindResources("*", "*", "Event", "")
	require.NoError(t, err)
	require.Len(t, resources, 2, "scope checks must still inspect both API groups")
}
