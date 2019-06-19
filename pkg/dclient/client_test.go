package client

import (
	"testing"

	policytypes "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	client  *Client
}

func newFixture(t *testing.T) *fixture {
	// init groupversion
	regResource := []schema.GroupVersionResource{
		schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"},
		schema.GroupVersionResource{Group: "group2", Version: "version", Resource: "thekinds"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	objects := []runtime.Object{newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-baz"),
		newUnstructured("apps/v1", "Deployment", "kyverno", "kyverno-deployment"),
	}
	scheme := runtime.NewScheme()
	// Create mock client
	client, err := NewMockClient(scheme, objects...)
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
	_, err := f.client.GetResource("thekinds", "ns-foo", "name-foo")
	if err != nil {
		t.Errorf("GetResource not working: %s", err)
	}
	// List Resources
	_, err = f.client.ListResource("thekinds", "ns-foo")
	if err != nil {
		t.Errorf("ListResource not working: %s", err)
	}
	// DeleteResouce
	err = f.client.DeleteResouce("thekinds", "ns-foo", "name-bar", false)
	if err != nil {
		t.Errorf("DeleteResouce not working: %s", err)
	}
	// CreateResource
	_, err = f.client.CreateResource("thekinds", "ns-foo", newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"), false)
	if err != nil {
		t.Errorf("CreateResource not working: %s", err)
	}
	//	UpdateResource
	_, err = f.client.UpdateResource("thekinds", "ns-foo", newUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "bar"}), false)
	if err != nil {
		t.Errorf("UpdateResource not working: %s", err)
	}
	// UpdateStatusResource
	_, err = f.client.UpdateStatusResource("thekinds", "ns-foo", newUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "status"}), false)
	if err != nil {
		t.Errorf("UpdateStatusResource not working: %s", err)
	}
}

func TestEventInterface(t *testing.T) {
	f := newFixture(t)
	iEvent, err := f.client.GetEventsInterface()
	if err != nil {
		t.Errorf("GetEventsInterface not working: %s", err)
	}
	_, err = iEvent.List(meta.ListOptions{})
	if err != nil {
		t.Errorf("Testing Event interface not working: %s", err)
	}
}
func TestCSRInterface(t *testing.T) {
	f := newFixture(t)
	iCSR, err := f.client.GetCSRInterface()
	if err != nil {
		t.Errorf("GetCSRInterface not working: %s", err)
	}
	_, err = iCSR.List(meta.ListOptions{})
	if err != nil {
		t.Errorf("Testing CSR interface not working: %s", err)
	}
}

func TestGenerateResource(t *testing.T) {
	f := newFixture(t)
	//GenerateResource -> copy From
	// 1 create namespace
	// 2 generate resource
	// create namespace
	ns, err := f.client.CreateResource("namespaces", "", newUnstructured("v1", "Namespace", "", "ns1"), false)
	if err != nil {
		t.Errorf("CreateResource not working: %s", err)
	}
	gen := policytypes.Generation{Kind: "TheKind",
		Name:  "gen-kind",
		Clone: &policytypes.CloneFrom{Namespace: "ns-foo", Name: "name-foo"}}
	err = f.client.GenerateResource(gen, ns.GetName())
	if err != nil {
		t.Errorf("GenerateResource not working: %s", err)
	}
	_, err = f.client.GetResource("thekinds", "ns1", "gen-kind")
	if err != nil {
		t.Errorf("GetResource not working: %s", err)
	}
	// GenerateResource -> data
	gen = policytypes.Generation{Kind: "TheKind",
		Name: "name2-baz-new",
		Data: newUnstructured("group2/version", "TheKind", "ns1", "name2-baz-new")}
	err = f.client.GenerateResource(gen, ns.GetName())
	if err != nil {
		t.Errorf("GenerateResource not working: %s", err)
	}
	_, err = f.client.GetResource("thekinds", "ns1", "name2-baz-new")
	if err != nil {
		t.Errorf("GetResource not working: %s", err)
	}
}

func TestKubePolicyDeployment(t *testing.T) {
	f := newFixture(t)
	_, err := f.client.GetKubePolicyDeployment()
	if err != nil {
		t.Fatal(err)
	}
}
