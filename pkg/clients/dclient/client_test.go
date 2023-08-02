package dclient

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetResource
// ListResource
// CreateResource
// getGroupVersionMapper (valid and invalid resources)

//NewMockClient creates a mock client
// - dynamic client
// - kubernetes client
// - objects to initialize the client

type fixture struct {
	t       *testing.T
	objects []runtime.Object
	client  Interface
}

func newFixture(t *testing.T) *fixture {
	// init groupversion
	regResource := []schema.GroupVersionResource{
		{Group: "group", Version: "version", Resource: "thekinds"},
		{Group: "group2", Version: "version", Resource: "thekinds"},
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "group", Version: "version", Resource: "thekinds"}:  "TheKindList",
		{Group: "group2", Version: "version", Resource: "thekinds"}: "TheKindList",
		{Group: "", Version: "v1", Resource: "namespaces"}:          "NamespaceList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:     "DeploymentList",
	}

	objects := []runtime.Object{
		kubeutils.NewUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		kubeutils.NewUnstructured("group2/version", "TheKind", "ns-foo", "name2-foo"),
		kubeutils.NewUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		kubeutils.NewUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		kubeutils.NewUnstructured("group2/version", "TheKind", "ns-foo", "name2-baz"),
		kubeutils.NewUnstructured("apps/v1", "Deployment", config.KyvernoNamespace(), config.KyvernoDeploymentName()),
	}

	scheme := runtime.NewScheme()
	// Create mock client
	client, err := NewFakeClient(scheme, gvrToListKind, objects...)
	if err != nil {
		t.Fatal(err)
	}

	// set discovery Client
	client.SetDiscovery(NewFakeDiscoveryClient(regResource))

	f := fixture{
		t:       t,
		objects: objects,
		client:  client,
	}
	return &f

}

func TestCRUDResource(t *testing.T) {
	f := newFixture(t)
	// Get Resource
	_, err := f.client.GetResource(context.TODO(), "", "thekind", "ns-foo", "name-foo")
	if err != nil {
		t.Errorf("GetResource not working: %s", err)
	}
	// List Resources
	_, err = f.client.ListResource(context.TODO(), "", "thekind", "ns-foo", nil)
	if err != nil {
		t.Errorf("ListResource not working: %s", err)
	}
	// DeleteResouce
	err = f.client.DeleteResource(context.TODO(), "", "thekind", "ns-foo", "name-bar", false)
	if err != nil {
		t.Errorf("DeleteResouce not working: %s", err)
	}
	// CreateResource
	_, err = f.client.CreateResource(context.TODO(), "", "thekind", "ns-foo", kubeutils.NewUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"), false)
	if err != nil {
		t.Errorf("CreateResource not working: %s", err)
	}
	//	UpdateResource
	_, err = f.client.UpdateResource(context.TODO(), "", "thekind", "ns-foo", kubeutils.NewUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "bar"}), false)
	if err != nil {
		t.Errorf("UpdateResource not working: %s", err)
	}
	// UpdateStatusResource
	_, err = f.client.UpdateStatusResource(context.TODO(), "", "thekind", "ns-foo", kubeutils.NewUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "status"}), false)
	if err != nil {
		t.Errorf("UpdateStatusResource not working: %s", err)
	}
}

func TestEventInterface(t *testing.T) {
	f := newFixture(t)
	iEvent := f.client.GetEventsInterface()
	_, err := iEvent.Events(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Errorf("Testing Event interface not working: %s", err)
	}
}
