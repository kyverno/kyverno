package generate

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func setup(t *testing.T) {
	RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
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
		deleteResources(client, expectedResource{resource.gvr, result.GetNamespace(), result.GetName()})
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
		deleteResources(client, expectedResource{resource.gvr, result.GetNamespace(), result.GetName()})
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

func expectClusteredResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s", resource.gvr.String(), resource.name))
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		_, err := client.GetClusteredResource(resource.gvr, resource.name)
		if err != nil {
			return err
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func expectNamespacedResource(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	err := e2e.GetWithRetry(1*time.Second, 30, func() error {
		_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
		if err != nil {
			return err
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
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
	_, err := client.GetClusteredResource(resource.gvr, resource.name)
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func expectNamespacedResourceNotExists(client *e2e.E2EClient, resource expectedResource) {
	By(fmt.Sprintf("Expecting not exists %s : %s/%s", resource.gvr.String(), resource.ns, resource.name))
	_, err := client.GetNamespacedResource(resource.gvr, resource.ns, resource.name)
	Expect(apierrors.IsNotFound(err)).To(BeTrue())
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
