package common

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsFilteredByConfig_KubeSystemNamespace(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{"resourceFilters": "[*/*,kube-system,*]"},
	})

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"})
	obj.SetName("kube-system")

	if !IsFilteredByConfig(cfg, obj) {
		t.Fatal("expected kube-system Namespace to be filtered")
	}
}

func TestIsFilteredByConfig_UnfilteredNamespace(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno", Namespace: "kyverno"},
		Data:       map[string]string{"resourceFilters": "[*/*,kube-system,*]"},
	})

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"})
	obj.SetName("default")

	if IsFilteredByConfig(cfg, obj) {
		t.Fatal("expected default Namespace not to be filtered")
	}
}

func TestIsFilteredByConfig_NilObject(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	if IsFilteredByConfig(cfg, nil) {
		t.Fatal("expected nil object not to be filtered")
	}
}
