package logging

import (
	"github.com/go-logr/logr"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type controller struct {
	logger logr.Logger
}

type informer interface {
	Informer() cache.SharedIndexInformer
}

func NewController(logger logr.Logger, objectType string, informer informer) {
	c := controller{
		logger: logger.WithValues("type", objectType),
	}
	controllerutils.AddEventHandlersT(informer.Informer(), c.add, c.update, c.delete)
}

func (c *controller) add(obj metav1.Object) {
	name, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name", "object", obj)
		name = "unknown"
	}
	c.logger.Info("resource added", "name", name)
}

func (c *controller) update(old, obj metav1.Object) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		name, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			c.logger.Error(err, "failed to extract name", "object", obj)
			name = "unknown"
		}
		c.logger.Info("resource updated", "name", name)
	}
}

func (c *controller) delete(obj metav1.Object) {
	name, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name", "object", obj)
		name = "unknown"
	}
	c.logger.Info("resource deleted", "name", name)
}
