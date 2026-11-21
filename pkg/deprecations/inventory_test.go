package deprecations

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newPartialObjectMetadata(apiVersion, kind, namespace, name string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func TestMetadataKindCounters(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme,
		newPartialObjectMetadata("kyverno.io/v1", "ClusterPolicy", "", "a"),
		newPartialObjectMetadata("kyverno.io/v1", "ClusterPolicy", "", "b"),
		newPartialObjectMetadata("kyverno.io/v1", "Policy", "default", "c"),
	)

	counters := MetadataKindCounters(client, "ClusterPolicy", "Policy")

	count, err := counters["ClusterPolicy"](context.Background())
	if err != nil {
		t.Fatalf("ClusterPolicy counter: unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("ClusterPolicy counter = %d, want 2", count)
	}

	count, err = counters["Policy"](context.Background())
	if err != nil {
		t.Fatalf("Policy counter: unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("Policy counter = %d, want 1", count)
	}
}

func TestMetadataKindCounters_NotFound(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme)
	client.PrependReactor("list", "clusterpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "")
	})

	counters := MetadataKindCounters(client, "ClusterPolicy")
	count, err := counters["ClusterPolicy"](context.Background())
	if err != nil {
		t.Fatalf("expected nil error on NotFound, got %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 on NotFound", count)
	}
}

func TestMetadataKindCounters_OtherError(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme)
	client.PrependReactor("list", "clusterpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "", nil)
	})

	counters := MetadataKindCounters(client, "ClusterPolicy")
	if _, err := counters["ClusterPolicy"](context.Background()); err == nil {
		t.Fatalf("expected an error, got nil")
	}
}

func TestMetadataKindCounters_UnknownKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected a panic for an unknown legacy policy kind")
		}
	}()
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme)
	MetadataKindCounters(client, "NotALegacyKind")
}
