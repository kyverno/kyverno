package utils

import (
	"context"

	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func Create[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, o *K, opts metav1.CreateOptions, inner func(context.Context, *K, metav1.CreateOptions) (*K, error)) (*K, error) {
	m.Record(metrics.ClientCreate, metrics.KyvernoClient, kind, ns)
	return inner(ctx, o, opts)
}

func Update[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, o *K, opts metav1.UpdateOptions, inner func(context.Context, *K, metav1.UpdateOptions) (*K, error)) (*K, error) {
	m.Record(metrics.ClientUpdate, metrics.KyvernoClient, kind, ns)
	return inner(ctx, o, opts)
}

func UpdateStatus[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, o *K, opts metav1.UpdateOptions, inner func(context.Context, *K, metav1.UpdateOptions) (*K, error)) (*K, error) {
	m.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, kind, ns)
	return inner(ctx, o, opts)
}

func Delete(ctx context.Context, m ClientQueryMetric, kind string, ns string, name string, opts metav1.DeleteOptions, inner func(context.Context, string, metav1.DeleteOptions) error) error {
	m.Record(metrics.ClientDelete, metrics.KyvernoClient, kind, ns)
	return inner(ctx, name, opts)
}

func DeleteCollection(ctx context.Context, m ClientQueryMetric, kind string, ns string, opts metav1.DeleteOptions, listOpts metav1.ListOptions, inner func(context.Context, metav1.DeleteOptions, metav1.ListOptions) error) error {
	m.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, kind, ns)
	return inner(ctx, opts, listOpts)
}

func Get[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, name string, opts metav1.GetOptions, inner func(context.Context, string, metav1.GetOptions) (*K, error)) (*K, error) {
	m.Record(metrics.ClientGet, metrics.KyvernoClient, kind, ns)
	return inner(ctx, name, opts)
}

func List[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, opts metav1.ListOptions, inner func(context.Context, metav1.ListOptions) (*K, error)) (*K, error) {
	m.Record(metrics.ClientList, metrics.KyvernoClient, kind, ns)
	return inner(ctx, opts)
}

func Watch(ctx context.Context, m ClientQueryMetric, kind string, ns string, opts metav1.ListOptions, inner func(context.Context, metav1.ListOptions) (watch.Interface, error)) (watch.Interface, error) {
	m.Record(metrics.ClientWatch, metrics.KyvernoClient, kind, ns)
	return inner(ctx, opts)
}

func Patch[K any](ctx context.Context, m ClientQueryMetric, kind string, ns string, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, inner func(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*K, error), subresources ...string) (*K, error) {
	m.Record(metrics.ClientPatch, metrics.KyvernoClient, kind, ns)
	return inner(ctx, name, pt, data, opts, subresources...)
}
