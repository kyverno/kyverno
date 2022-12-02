package resolvers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

type NamespacedResourceResolver[T any] interface {
	Get(context.Context, string, string) (T, error)
}

type ConfigmapResolver = NamespacedResourceResolver[*corev1.ConfigMap]
