package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/metadata"
	metadatafake "k8s.io/client-go/metadata/fake"
)

func TestGenerate_BelowThreshold_CreatesUpdateRequest(t *testing.T) {
	ctx := context.Background()

	// Create a mock ConfigMap and load it into the configuration
	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "10",
		},
	}
	cfg.Load(mockConfigMap)

	// metadata client with 1 existing UpdateRequest
	existingUR := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "UpdateRequest",
			APIVersion: "kyverno.io/v2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ur-1",
			Namespace: config.KyvernoNamespace(),
		},
	}

	// Create a scheme and add the metav1 types to it.
	scheme := runtime.NewScheme()
	metav1.AddMetaToScheme(scheme)
	metaClient := metadatafake.NewSimpleMetadataClient(scheme, existingUR)

	kyvernoClient := fake.NewSimpleClientset()

	gen := NewUpdateRequestGenerator(cfg, metaClient)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	_, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the UpdateRequest was actually created
	createdUR, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, "test-ur", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get created UpdateRequest: %v", err)
	}
	if createdUR == nil {
		t.Fatalf("expected UpdateRequest to be created, but it was not found")
	}
}

func TestGenerate_AtThreshold_SkipsCreation(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "1",
		},
	}
	cfg.Load(mockConfigMap)

	existingUR := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "UpdateRequest",
			APIVersion: "kyverno.io/v2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ur-1",
			Namespace: config.KyvernoNamespace(),
		},
	}

	// Create a scheme and add the metav1 types to it.
	scheme := runtime.NewScheme()
	metav1.AddMetaToScheme(scheme)
	metaClient := metadatafake.NewSimpleMetadataClient(scheme, existingUR)

	kyvernoClient := fake.NewSimpleClientset()
	gen := NewUpdateRequestGenerator(cfg, metaClient)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "should-not-be-created",
		},
	}

	result, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no UpdateRequest to be created when threshold is met or exceeded")
	}

	// Verify the UpdateRequest was NOT created
	urs, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list UpdateRequests: %v", err)
	}
	if len(urs.Items) > 0 {
		t.Fatalf("expected no UpdateRequests to be created, but found %d", len(urs.Items))
	}
}

// errorMetaClient is a mock for metadata.Interface that always returns an error on List.
type errorMetaClient struct{}

// Updated to return metadata.Getter to match the interface
func (e *errorMetaClient) Resource(gvr schema.GroupVersionResource) metadata.Getter {
	return &errorMetaResource{}
}

type errorMetaResource struct{}

func (e *errorMetaResource) Namespace(string) metadata.ResourceInterface {
	return e
}
func (e *errorMetaResource) List(context.Context, metav1.ListOptions) (*metav1.PartialObjectMetadataList, error) {
	return nil, errors.New("list failed")
}

// Get method signature to match metadata.Getter interface
func (e *errorMetaResource) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*metav1.PartialObjectMetadata, error) {
	return nil, errors.New("get failed")
}
func (e *errorMetaResource) Create(context.Context, *metav1.PartialObjectMetadata, metav1.CreateOptions) (*metav1.PartialObjectMetadata, error) {
	return nil, errors.New("create failed")
}
func (e *errorMetaResource) Update(context.Context, *metav1.PartialObjectMetadata, metav1.UpdateOptions) (*metav1.PartialObjectMetadata, error) {
	return nil, errors.New("update failed")
}

// Updated Delete method signature
func (e *errorMetaResource) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	return errors.New("delete failed")
}
func (e *errorMetaResource) DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error {
	return errors.New("delete collection failed")
}
func (e *errorMetaResource) Watch(context.Context, metav1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("watch failed")
}
func (e *errorMetaResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*metav1.PartialObjectMetadata, error) {
	return nil, errors.New("patch failed")
}

func TestGenerate_MetadataListError_ReturnsError(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewDefaultConfiguration(false)
	mockConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"updateRequestThreshold": "10",
		},
	}
	cfg.Load(mockConfigMap)

	metaClient := &errorMetaClient{}
	// Revert to using NewSimpleClientset
	kyvernoClient := fake.NewSimpleClientset()

	gen := NewUpdateRequestGenerator(cfg, metaClient)

	resource := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	result, err := gen.Generate(ctx, kyvernoClient, resource, logr.Discard())

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if result != nil {
		t.Fatalf("expected no UpdateRequest on error")
	}
}
