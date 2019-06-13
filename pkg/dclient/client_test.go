package client

import (
	"fmt"
	"testing"

	policytypes "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// GetResource
// ListResource
// CreateResource
// getGroupVersionMapper (valid and invalid resources)

//NewMockClient creates a mock client
// - dynamic client
// - kubernetes client
// - objects to initialize the client

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newUnstructuredWithSpec(apiVersion, kind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := newUnstructured(apiVersion, kind, namespace, name)
	u.Object["spec"] = spec
	return u
}

func TestClient(t *testing.T) {
	scheme := runtime.NewScheme()
	// init groupversion
	regresource := map[string]string{"group/version": "thekinds",
		"group2/version": "thekinds",
		"v1":             "namespaces",
		"apps/v1":        "deployments"}
	// init resources
	objects := []runtime.Object{newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-baz"),
		newUnstructured("apps/v1", "Deployment", "kyverno", "kyverno-deployment"),
	}

	// Mock Client
	client, err := NewMockClient(scheme, objects...)
	if err != nil {
		t.Fatal(err)
	}

	// set discovery Client
	client.SetDiscovery(NewFakeDiscoveryClient(regresource))
	// Get Resource
	res, err := client.GetResource("thekinds", "ns-foo", "name-foo")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(res)
	// List Resources
	list, err := client.ListResource("thekinds", "ns-foo")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(len(list.Items))
	// DeleteResouce
	err = client.DeleteResouce("thekinds", "ns-foo", "name-bar")
	if err != nil {
		t.Fatal(err)
	}
	// CreateResource
	res, err = client.CreateResource("thekinds", "ns-foo", newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"))
	if err != nil {
		t.Fatal(err)
	}
	//	UpdateResource
	res, err = client.UpdateResource("thekinds", "ns-foo", newUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "bar"}))
	if err != nil {
		t.Fatal(err)
	}

	// UpdateStatusResource
	res, err = client.UpdateStatusResource("thekinds", "ns-foo", newUnstructuredWithSpec("group/version", "TheKind", "ns-foo", "name-foo1", map[string]interface{}{"foo": "status"}))
	if err != nil {
		t.Fatal(err)
	}

	iEvent, err := client.GetEventsInterface()
	if err != nil {
		t.Fatal(err)
	}
	eventList, err := iEvent.List(meta.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(eventList.Items)

	iCSR, err := client.GetCSRInterface()
	if err != nil {
		t.Fatal(err)
	}
	csrList, err := iCSR.List(meta.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(csrList.Items)

	//GenerateResource -> copy From
	// 1 create namespace
	// 2 generate resource

	// create namespace
	ns, err := client.CreateResource("namespaces", "", newUnstructured("v1", "Namespace", "", "ns1"))
	if err != nil {
		t.Fatal(err)
	}
	gen := policytypes.Generation{Kind: "TheKind",
		Name:  "gen-kind",
		Clone: &policytypes.CloneFrom{Namespace: "ns-foo", Name: "name-foo"}}
	err = client.GenerateResource(gen, ns.GetName())
	if err != nil {
		t.Fatal(err)
	}
	res, err = client.GetResource("thekinds", "ns1", "gen-kind")
	if err != nil {
		t.Fatal(err)
	}
	// GenerateResource -> data
	gen = policytypes.Generation{Kind: "TheKind",
		Name: "name2-baz-new",
		Data: newUnstructured("group2/version", "TheKind", "ns1", "name2-baz-new")}
	err = client.GenerateResource(gen, ns.GetName())
	if err != nil {
		t.Fatal(err)
	}
	res, err = client.GetResource("thekinds", "ns1", "name2-baz-new")
	if err != nil {
		t.Fatal(err)
	}

	// Get Kube Policy Deployment
	deploy, err := client.GetKubePolicyDeployment()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(deploy.GetName())
}
