package main

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/cmd/internal"
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

func TestCountLegacyPolicies_Present(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme,
		newPartialObjectMetadata("kyverno.io/v1", "ClusterPolicy", "", "a"),
		newPartialObjectMetadata("kyverno.io/v1", "Policy", "default", "b"),
	)

	counts, err := countLegacyPolicies(context.Background(), internal.SetupResult{MetadataClient: client})
	if err != nil {
		t.Fatalf("countLegacyPolicies() unexpected error: %v", err)
	}
	if counts["ClusterPolicy"] != 1 {
		t.Errorf("counts[ClusterPolicy] = %d, want 1", counts["ClusterPolicy"])
	}
	if counts["Policy"] != 1 {
		t.Errorf("counts[Policy] = %d, want 1", counts["Policy"])
	}
	if counts["CleanupPolicy"] != 0 {
		t.Errorf("counts[CleanupPolicy] = %d, want 0", counts["CleanupPolicy"])
	}
}

func TestCountLegacyPolicies_CRDAbsent(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme)
	for _, resource := range []string{"clusterpolicies", "policies", "cleanuppolicies", "clustercleanuppolicies", "policyexceptions"} {
		resource := resource
		client.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: resource}, "")
		})
	}

	counts, err := countLegacyPolicies(context.Background(), internal.SetupResult{MetadataClient: client})
	if err != nil {
		t.Fatalf("countLegacyPolicies() unexpected error: %v", err)
	}
	for kind, count := range counts {
		if count != 0 {
			t.Errorf("counts[%s] = %d, want 0 when the CRD is absent", kind, count)
		}
	}
}

func TestCountLegacyPolicies_ListErrored(t *testing.T) {
	scheme := fake.NewTestScheme()
	metav1.AddMetaToScheme(scheme)
	client := fake.NewSimpleMetadataClient(scheme)
	client.PrependReactor("list", "clusterpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "", nil)
	})

	if _, err := countLegacyPolicies(context.Background(), internal.SetupResult{MetadataClient: client}); err == nil {
		t.Fatalf("countLegacyPolicies() expected an error when a list is forbidden, got nil")
	}
}
