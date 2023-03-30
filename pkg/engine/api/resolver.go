package api

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
)

// NamespacedResourceResolver is an abstract interface used to resolve namespaced resources
// Any implementation might exist, cache based, file based, client based etc...
type NamespacedResourceResolver[T any] interface {
	// Get is used to resolve a resource given a namespace and name
	Get(
		ctx context.Context,
		namespace string,
		name string,
	) (T, error)
}

// ConfigmapResolver is an abstract interface used to resolve configmaps
type ConfigmapResolver = NamespacedResourceResolver[*corev1.ConfigMap]

// namespacedResourceResolverChain represents a chain of NamespacedResourceResolver
type namespacedResourceResolverChain[T any] []NamespacedResourceResolver[T]

// NewNamespacedResourceResolver creates a NamespacedResourceResolver from a NamespacedResourceResolver chain
// It will try to resolve resources by iterating over individual resolvers until one finds the requested resource
func NewNamespacedResourceResolver[T any](resolvers ...NamespacedResourceResolver[T]) (NamespacedResourceResolver[T], error) {
	if len(resolvers) == 0 {
		return nil, errors.New("no resolvers")
	}
	for _, resolver := range resolvers {
		if resolver == nil {
			return nil, errors.New("at least one resolver is nil")
		}
	}
	return namespacedResourceResolverChain[T](resolvers), nil
}

func (chain namespacedResourceResolverChain[T]) Get(ctx context.Context, namespace, name string) (T, error) {
	var lastErr error
	for _, resolver := range chain {
		r, err := resolver.Get(ctx, namespace, name)
		if err == nil {
			return r, nil
		}
		lastErr = err
	}
	var notFound T
	return notFound, lastErr
}
