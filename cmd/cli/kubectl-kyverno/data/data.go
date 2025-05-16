package data

import (
	"embed"
	"encoding/json"
	"io/fs"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/restmapper"
)

const crdsFolder = "crds"

//go:embed crds
var crdsFs embed.FS

//go:embed api-group-resources.json
var apiGroupResources []byte

//go:embed deprecated-apis.json
var deprecatedAPIs []byte

func Crds() (fs.FS, error) {
	return fs.Sub(crdsFs, crdsFolder)
}

func mergeGroupResources(gr1, gr2 []*restmapper.APIGroupResources) []*restmapper.APIGroupResources {
	groupMap := make(map[string]*restmapper.APIGroupResources)

	addOrMerge := func(in *restmapper.APIGroupResources, isDeprecated bool) {
		group := in.Group.Name
		existing, found := groupMap[group]
		if !found {
			// Copy versions and resources
			groupMap[group] = &restmapper.APIGroupResources{
				Group: metav1.APIGroup{
					Name:     in.Group.Name,
					Versions: append([]metav1.GroupVersionForDiscovery{}, in.Group.Versions...),
				},
				VersionedResources: copyVersionedResources(in.VersionedResources),
			}
			// Set preferred version only if NOT deprecated
			if !isDeprecated {
				groupMap[group].Group.PreferredVersion = in.Group.PreferredVersion
			}
			return
		}

		// Merge versions (avoid duplicates)
		existing.Group.Versions = appendUniqueVersions(existing.Group.Versions, in.Group.Versions)

		// Merge versioned resources (avoid duplicates)
		for version, resources := range in.VersionedResources {
			existing.VersionedResources[version] = appendUniqueResources(existing.VersionedResources[version], resources)
		}

		// Only set preferred version if existing is empty and NOT deprecated
		if existing.Group.PreferredVersion.Version == "" && !isDeprecated && in.Group.PreferredVersion.Version != "" {
			existing.Group.PreferredVersion = in.Group.PreferredVersion
		}
	}

	for _, g := range gr1 {
		addOrMerge(g, false) // stable
	}
	for _, g := range gr2 {
		addOrMerge(g, true) // deprecated, ignore preferred version
	}
	for _, group := range groupMap {
		preferred := group.Group.PreferredVersion.Version
		stableResources, hasStable := group.VersionedResources[preferred]
		if !hasStable || len(stableResources) == 0 {
			continue
		}

		for _, gv := range group.Group.Versions {
			v := gv.Version
			if len(group.VersionedResources[v]) == 0 && v != preferred {
				group.VersionedResources[v] = append([]metav1.APIResource{}, stableResources...)
			}
		}
	}

	merged := make([]*restmapper.APIGroupResources, 0, len(groupMap))
	for _, v := range groupMap {
		merged = append(merged, v)
	}
	return merged
}

var _apiGroupResources = sync.OnceValues(func() ([]*restmapper.APIGroupResources, error) {
	var a1, a2 []*restmapper.APIGroupResources

	if err := json.Unmarshal(apiGroupResources, &a1); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(deprecatedAPIs, &a2); err != nil {
		return nil, err
	}

	return mergeGroupResources(a1, a2), nil
})

func APIGroupResources() ([]*restmapper.APIGroupResources, error) {
	return _apiGroupResources()
}

func appendUniqueVersions(dst, src []metav1.GroupVersionForDiscovery) []metav1.GroupVersionForDiscovery {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[v.Version] = true
	}
	for _, v := range src {
		if !seen[v.Version] {
			dst = append(dst, v)
			seen[v.Version] = true
		}
	}
	return dst
}

func appendUniqueResources(dst, src []metav1.APIResource) []metav1.APIResource {
	seen := map[string]bool{}
	for _, r := range dst {
		seen[r.Name] = true
	}
	for _, r := range src {
		if !seen[r.Name] {
			dst = append(dst, r)
			seen[r.Name] = true
		}
	}
	return dst
}

func copyVersionedResources(in map[string][]metav1.APIResource) map[string][]metav1.APIResource {
	out := make(map[string][]metav1.APIResource)
	for version, resources := range in {
		out[version] = append([]metav1.APIResource{}, resources...)
	}
	return out
}
