package utils

import (
	"context"
	"io"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mockResourceClient struct {
	resources map[string]*unstructured.Unstructured
}

func (m *mockResourceClient) GetResource(_ context.Context, _ string, kind string, namespace string, name string, _ ...string) (*unstructured.Unstructured, error) {
	key := kind + "/" + namespace + "/" + name
	if obj, ok := m.resources[key]; ok {
		return obj, nil
	}
	return &unstructured.Unstructured{}, nil
}
func (m *mockResourceClient) ListResource(_ context.Context, _ string, _ string, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, nil
}
func (m *mockResourceClient) GetResources(_ context.Context, _, _, _, _, _, _ string, _ *metav1.LabelSelector) ([]engineapi.Resource, error) {
	return nil, nil
}
func (m *mockResourceClient) GetNamespace(_ context.Context, _ string, _ metav1.GetOptions) (*corev1.Namespace, error) {
	return nil, nil
}
func (m *mockResourceClient) IsNamespaced(_, _, _ string) (bool, error) {
	return true, nil
}
func (m *mockResourceClient) CanI(_ context.Context, _, _, _, _, _ string) (bool, string, error) {
	return true, "", nil
}
func (m *mockResourceClient) RawAbsPath(_ context.Context, _ string, _ string, _ io.Reader) ([]byte, error) {
	return nil, nil
}

func makeTestUnstructured(kind, namespace, name string, ts metav1.Time, ownerKind, ownerName string, isController bool) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetCreationTimestamp(ts)
	if ownerName != "" {
		controller := isController
		obj.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: ownerKind, Name: ownerName, Controller: &controller}})
	}
	return obj
}

func TestRootOwnerCreationTimestamp_NoOwner(t *testing.T) {
	ts := metav1.Unix(1000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", ts, "", "", false)
	client := &mockResourceClient{}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(&ts) {
		t.Errorf("expected pod timestamp, got %v", got)
	}
}

func TestRootOwnerCreationTimestamp_WalksToRoot(t *testing.T) {
	deployTs := metav1.Unix(1000, 0)
	rsTs := metav1.Unix(2000, 0)
	podTs := metav1.Unix(3000, 0)
	deploy := makeTestUnstructured("Deployment", "default", "deploy1", deployTs, "", "", false)
	rs := makeTestUnstructured("ReplicaSet", "default", "rs1", rsTs, "Deployment", "deploy1", true)
	pod := makeTestUnstructured("Pod", "default", "pod1", podTs, "ReplicaSet", "rs1", true)
	client := &mockResourceClient{resources: map[string]*unstructured.Unstructured{"ReplicaSet/default/rs1": &rs, "Deployment/default/deploy1": &deploy}}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(&deployTs) {
		t.Errorf("expected deploy timestamp %v, got %v", deployTs, got)
	}
}

func TestRootOwnerCreationTimestamp_NonControllerOwnerIgnored(t *testing.T) {
	podTs := metav1.Unix(3000, 0)
	pod := makeTestUnstructured("Pod", "default", "pod1", podTs, "ReplicaSet", "rs1", false)
	client := &mockResourceClient{}
	got, err := RootOwnerCreationTimestamp(context.TODO(), client, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(&podTs) {
		t.Errorf("expected pod timestamp, got %v", got)
	}
}
