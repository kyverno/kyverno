package generate

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type resource struct {
	gvr schema.GroupVersionResource
	ns  string
	raw []byte
}

func clusteredResource(gvr schema.GroupVersionResource, raw []byte) resource {
	return resource{gvr, "", raw}
}

func namespacedResource(gvr schema.GroupVersionResource, ns string, raw []byte) resource {
	return resource{gvr, ns, raw}
}

type existingResource struct {
	gvr  schema.GroupVersionResource
	ns   string
	name string
}

func existing(gvr schema.GroupVersionResource, ns string, name string) existingResource {
	return existingResource{gvr, ns, name}
}

type expectedResource struct {
	existingResource
	validate []func(*unstructured.Unstructured)
}

func expected(gvr schema.GroupVersionResource, ns string, name string, validate ...func(*unstructured.Unstructured)) expectedResource {
	return expectedResource{existing(gvr, ns, name), validate}
}

func setup(t *testing.T) {
	RegisterTestingT(t)
	// if os.Getenv("E2E") == "" {
	// 	t.Skip("Skipping E2E Test")
	// }
}

func createClient() *e2e.E2EClient {
	client, err := e2e.NewE2EClient()
	Expect(err).NotTo(HaveOccurred())
	return client
}

func deleteClusteredResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Deleting %s : %s", resource.gvr.String(), resource.name))
	client.DeleteClusteredResource(resource.gvr, resource.name)
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		if err == nil {
			return fmt.Errorf("resource still exists: %s, %s", resource.gvr.String(), resource.name)
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	Expect(err).NotTo(HaveOccurred())
}

func deleteNamespacedResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Deleting %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	client.DeleteNamespacedResource(resource.gvr, resource.ns, resource.name)
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
		if err == nil {
			return fmt.Errorf("resource still exists: %s, %s/%s", resource.gvr.String(), resource.ns, resource.name)
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	Expect(err).NotTo(HaveOccurred())
}

func deleteResource(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		deleteNamespacedResource(client, resource)
	} else {
		deleteClusteredResource(client, resource)
	}
}

func deleteResources(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		deleteResource(client, resource)
	}
}

func createClusteredResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	var u unstructured.Unstructured
	Expect(yaml.Unmarshal(resource.raw, &u)).To(Succeed())
	By(fmt.Sprintf("Creating %s : %s", resource.gvr.String(), u.GetName()))
	result, err := client.CreateClusteredResource(resource.gvr, &u)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		deleteResources(client, expected(resource.gvr, result.GetNamespace(), result.GetName()))
	})
	return result
}

func createNamespacedResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	var u unstructured.Unstructured
	Expect(yaml.Unmarshal(resource.raw, &u)).To(Succeed())
	By(fmt.Sprintf("Creating %s : %s/%s", resource.gvr.String(), resource.ns, u.GetName()))
	result, err := client.CreateNamespacedResource(resource.gvr, resource.ns, &u)
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		deleteResources(client, expected(resource.gvr, result.GetNamespace(), result.GetName()))
	})
	return result
}

func createResource(t *testing.T, client *e2e.E2EClient, resource resource) *unstructured.Unstructured {
	if resource.ns != "" {
		return createNamespacedResource(t, client, resource)
	} else {
		return createClusteredResource(t, client, resource)
	}
}

func createResources(t *testing.T, client *e2e.E2EClient, resources ...resource) {
	for _, resource := range resources {
		createResource(t, client, resource)
	}
}

func getClusteredResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, name string) *unstructured.Unstructured {
	By(fmt.Sprintf("Getting %s : %s", gvr.String(), name))
	r, err := client.GetClusteredResource(gvr, name)
	Expect(err).NotTo(HaveOccurred())
	return r
}

func getNamespacedResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, ns, name string) *unstructured.Unstructured {
	By(fmt.Sprintf("Getting %s : %s/%s", gvr.String(), ns, name))
	r, err := client.GetNamespacedResource(gvr, ns, name)
	Expect(err).NotTo(HaveOccurred())
	return r
}

func getResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, ns, name string) *unstructured.Unstructured {
	if ns != "" {
		return getNamespacedResource(client, gvr, ns, name)
	} else {
		return getClusteredResource(client, gvr, name)
	}
}

func updateClusteredResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, name string, m func(*unstructured.Unstructured) error) *unstructured.Unstructured {
	r := getClusteredResource(client, gvr, name)
	version := r.GetResourceVersion()
	Expect(m(r)).To(Succeed())
	By(fmt.Sprintf("Updating %s : %s", gvr.String(), name))
	r.SetResourceVersion(version)
	r, err := client.UpdateClusteredResource(gvr, r)
	Expect(err).NotTo(HaveOccurred())
	return r
}

func updateNamespacedResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, ns, name string, m func(*unstructured.Unstructured) error) *unstructured.Unstructured {
	r := getNamespacedResource(client, gvr, ns, name)
	version := r.GetResourceVersion()
	Expect(m(r)).To(Succeed())
	By(fmt.Sprintf("Updating %s : %s/%s", gvr.String(), ns, name))
	r.SetResourceVersion(version)
	r, err := client.UpdateNamespacedResource(gvr, ns, r)
	Expect(err).NotTo(HaveOccurred())
	return r
}

func updateResource(client *e2e.E2EClient, gvr schema.GroupVersionResource, ns, name string, m func(*unstructured.Unstructured) error) *unstructured.Unstructured {
	if ns != "" {
		return updateNamespacedResource(client, gvr, ns, name, m)
	} else {
		return updateClusteredResource(client, gvr, name, m)
	}
}

func expectClusteredResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s", resource.gvr.String(), resource.name))
	var r *unstructured.Unstructured
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		get, err := client.GetClusteredResource(resource.gvr, resource.name)
		if err != nil {
			return err
		}
		r = get
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(r).NotTo(BeNil())
	for _, v := range resource.validate {
		v(r)
	}
}

func expectNamespacedResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	var r *unstructured.Unstructured
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		get, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
		if err != nil {
			return err
		}
		r = get
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(r).NotTo(BeNil())
	for _, v := range resource.validate {
		v(r)
	}
}

func expectResource(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		expectNamespacedResource(client, resource)
	} else {
		expectClusteredResource(client, resource)
	}
}

func expectResources(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		expectResource(client, resource)
	}
}

func expectClusteredResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting not exists %s : %s", resource.gvr.String(), resource.name))
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		return err
	})
	Expect(err).To(HaveOccurred())
}

func expectNamespacedResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting not exists %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		return err
	})
	Expect(err).To(HaveOccurred())
}

func expectResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	if resource.ns != "" {
		expectNamespacedResourceNotExists(client, resource)
	} else {
		expectClusteredResourceNotExists(client, resource)
	}
}

func expectResourcesNotExist(client *e2e.E2EClient, resources ...expectedResource) {
	for _, resource := range resources {
		expectResourceNotExists(client, resource)
	}
}

type testCaseStep func(*e2e.E2EClient) error

type resourceExpectation func(resource *unstructured.Unstructured)

func stepDeleteResource(gvr schema.GroupVersionResource, ns string, name string) testCaseStep {
	return func(client *e2e.E2EClient) error {
		deleteResource(client, expected(gvr, ns, name))
		return nil
	}
}

func stepExpectResource(gvr schema.GroupVersionResource, ns string, name string, validate ...func(*unstructured.Unstructured)) testCaseStep {
	return func(client *e2e.E2EClient) error {
		expectResource(client, expected(gvr, ns, name, validate...))
		return nil
	}
}

func stepUpateResource(gvr schema.GroupVersionResource, ns, name string, m func(*unstructured.Unstructured) error) testCaseStep {
	return func(client *e2e.E2EClient) error {
		updateResource(client, gvr, ns, name, m)
		return nil
	}
}

func stepResourceNotFound(gvr schema.GroupVersionResource, ns string, name string) testCaseStep {
	return func(client *e2e.E2EClient) error {
		expectResourceNotExists(client, expected(gvr, ns, name))
		return nil
	}
}
