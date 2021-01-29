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
	paths := strings.Split(path, "/")

	if len(paths) < 4 || len(paths) > 7 {
		return nil, fmt.Errorf("invalid path %s", path)
	}

	if paths[0] != "api" && paths[0] != "apis" {
		return nil, fmt.Errorf("urlPath must start with /api/v1/ or /apis")
	}

	if paths[0] == "api" && paths[1] != "v1" {
		return nil, fmt.Errorf("urlPath must start with /api/v1/ or /apis")
	}

	// /apis/GROUP/VERSION/RESOURCETYPE/
	if len(paths) == 4 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[2],
			ResourceType: paths[3],
		}, nil
	}

	// /apis/GROUP/VERSION/RESOURCETYPE/NAME
	if len(paths) == 5 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[2],
			ResourceType: paths[3],
			Name:         paths[4],
		}, nil
	}

	// /apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE
	if len(paths) == 6 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[2],
			Namespace:    paths[4],
			ResourceType: paths[5],
		}, nil
	}

	// /apis/GROUP/VERSION/namespaces/NAMESPACE/RESOURCETYPE/NAME
	if len(paths) == 7 {
		return &APIPath{
			Root:         paths[0],
			Group:        paths[1],
			Version:      paths[2],
			Namespace:    paths[4],
			ResourceType: paths[5],
			Name:         paths[6],
		}, nil
	}

	return nil, fmt.Errorf("invalid path %s", path)
}

func (a *APIPath) String() string {
	if a.Namespace != "" {
		if a.Name == "" {
			paths := []string{a.Root, a.Group, a.Version, a.ResourceType, "namespaces", a.Namespace}
			return strings.Join(paths, "/")
		}

		paths := []string{a.Root, a.Group, a.Version, a.ResourceType, "namespaces", a.Namespace, a.Name}
		return strings.Join(paths, "/")
	}

	if a.Name != "" {
		return strings.Join([]string{a.Root, a.Group, a.Version, a.ResourceType, a.Name}, "/")
	}

	return strings.Join([]string{a.Root, a.Group, a.Version, a.ResourceType}, "/")
}
