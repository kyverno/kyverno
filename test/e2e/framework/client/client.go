package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyverno/kyverno/test/e2e"
	"github.com/kyverno/kyverno/test/e2e/framework/id"
	"github.com/kyverno/kyverno/test/e2e/framework/resource"
	"github.com/kyverno/kyverno/test/e2e/framework/utils"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Client interface {
	CreateResource(resource.Resource) *unstructured.Unstructured
	GetResource(id.Id) *unstructured.Unstructured
	DeleteResource(id.Id)
}

type client struct {
	t      *testing.T
	client *e2e.E2EClient
}

func New(t *testing.T) Client {
	t.Helper()
	c, err := e2e.NewE2EClient()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return &client{t, c}
}

func (c *client) CreateResource(resource resource.Resource) *unstructured.Unstructured {
	u := resource.Unstructured()
	ginkgo.By(fmt.Sprintf("Creating %s : %s", resource.Gvr(), utils.Key(u)))
	var err error
	if u.GetNamespace() != "" {
		u, err = c.client.CreateNamespacedResource(resource.Gvr(), u.GetNamespace(), u)
	} else {
		u, err = c.client.CreateClusteredResource(resource.Gvr(), u)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	c.t.Cleanup(func() {
		c.DeleteResource(id.New(resource.Gvr(), u.GetNamespace(), u.GetName()))
	})
	return u
}

func (c *client) DeleteResource(id id.Id) {
	ginkgo.By(fmt.Sprintf("Deleting %s : %s", id.GetGvr(), utils.Key(id)))
	var err error
	if id.IsClustered() {
		err = c.client.DeleteClusteredResource(id.GetGvr(), id.GetName())
	} else {
		err = c.client.DeleteNamespacedResource(id.GetGvr(), id.GetNamespace(), id.GetName())
	}
	if !apierrors.IsNotFound(err) {
		err = e2e.GetWithRetry(1*time.Second, 15, func() error {
			if id.IsClustered() {
				_, err = c.client.GetClusteredResource(id.GetGvr(), id.GetName())
			} else {
				_, err = c.client.GetNamespacedResource(id.GetGvr(), id.GetNamespace(), id.GetName())
			}
			if err == nil {
				return fmt.Errorf("resource still exists: %s, %s", id.GetGvr(), utils.Key(id))
			}
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		})
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func (c *client) GetResource(id id.Id) *unstructured.Unstructured {
	ginkgo.By(fmt.Sprintf("Getting %s : %s", id.GetGvr(), utils.Key(id)))
	var u *unstructured.Unstructured
	err := e2e.GetWithRetry(1*time.Second, 15, func() error {
		var err error
		if id.IsClustered() {
			u, err = c.client.GetClusteredResource(id.GetGvr(), id.GetName())
		} else {
			u, err = c.client.GetNamespacedResource(id.GetGvr(), id.GetNamespace(), id.GetName())
		}
		return err
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return u
}
