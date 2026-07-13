package admissionpolicy

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func BenchmarkCustomNamespaceListerList(b *testing.B) {
	namespaces := make([]*corev1.Namespace, 0, 100)
	for i := 0; i < cap(namespaces); i++ {
		namespaces = append(namespaces, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("benchmark-ns-%03d", i),
				Labels: map[string]string{
					"team": "kyverno",
					"env":  "perf",
				},
			},
		})
	}

	k8sClient := fake.NewClientset(namespacesToRuntimeObjects(namespaces)...)
	lister := NewCustomNamespaceLister(&mockDClient{kubeClient: k8sClient})
	selector := labels.Everything()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		list, err := lister.List(selector)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
		if len(list) != len(namespaces) {
			b.Fatalf("List() returned %d namespaces, want %d", len(list), len(namespaces))
		}
	}
}

func namespacesToRuntimeObjects(namespaces []*corev1.Namespace) []runtime.Object {
	objects := make([]runtime.Object, 0, len(namespaces))
	for _, namespace := range namespaces {
		objects = append(objects, namespace)
	}
	return objects
}
