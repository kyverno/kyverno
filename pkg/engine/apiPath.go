package engine

import (
	"fmt"
	"strings"
)

type APIPath struct {
	Root         string
	Group        string
	Version      string
	ResourceType string
	Name         string
	Namespace    string
}

// NewAPIPath validates and parses an API path.
// See: https://kubernetes.io/docs/reference/using-api/api-concepts/
func NewAPIPath(path string) (*APIPath, error) {
	trimmedPath := strings.Trim(path, "/ ")
	paths := strings.Split(trimmedPath, "/")

	if len(paths) < 3 || len(paths) > 7 {
		return nil, fmt.Errorf("invalid path length %s", path)
	}

	if paths[0] != "api" && paths[0] != "apis" {
		return nil, fmt.Errorf("urlPath must start with /api or /apis")
	}

	if paths[0] == "api" && paths[1] != "v1" {
		return nil, fmt.Errorf("expected urlPath to start with /api/v1/")
	}

	if paths[0] == "api" {

		// /api/v1/namespaces
		if len(paths) == 3 {
			return &APIPath{
				Root:         paths[0],
				Group:        paths[1],
				ResourceType: paths[2],
			}, nil
		}

		// /api/v1/namespaces/foo
		if len(paths) == 4 {
			return &APIPath{
				Root:         paths[0],
				Group:        paths[1],
				ResourceType: paths[2],
				Name:         paths[3],
			}, nil
		}

		// /api/v1/namespaces/foo/pods
		if len(paths) == 5 {
			return &APIPath{
				Root:         paths[0],
				Group:        paths[1],
				Namespace:    paths[3],
				ResourceType: paths[4],
			}, nil
		}

		if len(paths) == 6 {
			return &APIPath{
				Root:         paths[0],
				Group:        paths[1],
				Namespace:    paths[3],
				ResourceType: paths[4],
				Name:         paths[5],
			}, nil
		}

		return nil, fmt.Errorf("invalid API v1 path %s", path)
	}

	// /apis/GROUP/VERSION/RESOURCETYPE/
	if len(paths) == 4 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[1] + "/" + paths[2],
			ResourceType: paths[3],
		}, nil
	}

	// /apis/GROUP/VERSION/RESOURCETYPE/NAME
	if len(paths) == 5 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[1] + "/" + paths[2],
			ResourceType: paths[3],
			Name:         paths[4],
		}, nil
	}

	// /apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE
	if len(paths) == 6 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[1] + "/" + paths[2],
			Namespace:    paths[4],
			ResourceType: paths[5],
		}, nil
	}

	// /apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE/NAME
	if len(paths) == 7 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[1] + "/" + paths[2],
			Namespace:    paths[4],
			ResourceType: paths[5],
			Name:         paths[6],
		}, nil
	}

	return nil, fmt.Errorf("invalid API path %s", path)
}

func (a *APIPath) String() string {
	var paths []string
	if a.Root == "apis" {
		if a.Namespace != "" {
			if a.Name == "" {
				paths = []string{a.Root, a.Version, "namespaces", a.Namespace, a.ResourceType}
			} else {
				paths = []string{a.Root, a.Version, "namespaces", a.Namespace, a.ResourceType, a.Name}
			}
		} else {
			if a.Name != "" {
				paths = []string{a.Root, a.Version, a.ResourceType, a.Name}
			} else {
				paths = []string{a.Root, a.Version, a.ResourceType}
			}
		}
	} else {
		if a.Namespace != "" {
			if a.Name == "" {
				paths = []string{a.Root, a.Group, "namespaces", a.Namespace, a.ResourceType}
			} else {
				paths = []string{a.Root, a.Group, "namespaces", a.Namespace, a.ResourceType, a.Name}
			}
		} else {
			if a.Name != "" {
				paths = []string{a.Root, a.Group, a.ResourceType, a.Name}
			} else {
				paths = []string{a.Root, a.Group, a.ResourceType}
			}
		}
	}

	result := "/" + strings.Join(paths, "/")
	result = strings.ReplaceAll(result, "//", "/")
	return result
}
