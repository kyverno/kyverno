package engine

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
)

type NamespaceResolver = func(string) *corev1.Namespace

func NamespaceResolverWithTrace(resolver NamespaceResolver) NamespaceResolver {
	return func(namespace string) *corev1.Namespace {
		return tracing.Span1(context.Background(), "", "namespaceResolver", func(ctx context.Context, s trace.Span) *corev1.Namespace {
			return resolver(namespace)
		})
	}
}
