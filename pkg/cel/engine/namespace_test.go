package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func newNamespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func newNamespaceLister(t *testing.T, namespaces ...*corev1.Namespace) corev1listers.NamespaceLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, ns := range namespaces {
		assert.NoError(t, indexer.Add(ns))
	}
	return corev1listers.NewNamespaceLister(indexer)
}

func TestNewNamespaceResolver(t *testing.T) {
	tests := []struct {
		name      string
		lister    []*corev1.Namespace
		client    []*corev1.Namespace
		namespace string
		want      *corev1.Namespace
	}{{
		name:      "cache hit",
		lister:    []*corev1.Namespace{newNamespace("foo", map[string]string{"env": "prod"})},
		namespace: "foo",
		want:      newNamespace("foo", map[string]string{"env": "prod"}),
	}, {
		name:      "cache miss with live fallback",
		client:    []*corev1.Namespace{newNamespace("foo", map[string]string{"env": "prod"})},
		namespace: "foo",
		want:      newNamespace("foo", map[string]string{"env": "prod"}),
	}, {
		name:      "namespace not found",
		namespace: "foo",
		want:      nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]runtime.Object, 0, len(tt.client))
			for _, ns := range tt.client {
				objects = append(objects, ns)
			}
			client := fake.NewSimpleClientset(objects...)
			resolver := NewNamespaceResolver(klog.Background(), newNamespaceLister(t, tt.lister...), client)
			got := resolver(tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}
