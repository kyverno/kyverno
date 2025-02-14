package logging

import (
	"github.com/go-logr/logr"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Predicate = func(metav1.Object, metav1.Object) bool

func CheckVersion(old, obj metav1.Object) bool {
	return old.GetResourceVersion() != obj.GetResourceVersion()
}

func CheckGeneration(old, obj metav1.Object) bool {
	return old.GetGeneration() != obj.GetGeneration()
}

type controller struct {
	logger     logr.Logger
	predicates []Predicate
}

type informer interface {
	Informer() cache.SharedIndexInformer
}

func NewController(logger logr.Logger, objectType string, informer informer, predicates ...Predicate) {
	c := controller{
		logger:     logger.WithValues("type", objectType),
		predicates: predicates,
	}
	if _, err := controllerutils.AddEventHandlersT(informer.Informer(), c.add, c.update, c.delete); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
}

func (c *controller) add(obj metav1.Object) {
	name, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name", "object", obj)
		name = "unknown"
	}
	c.logger.V(2).Info("resource added", "name", name)
}

func (c *controller) update(old, obj metav1.Object) {
	for _, predicate := range c.predicates {
		if !predicate(old, obj) {
			return
		}
	}
	name, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name", "object", obj)
		name = "unknown"
	}
	c.logger.V(2).Info("resource updated", "name", name)
}

func (c *controller) delete(obj metav1.Object) {
	name, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Error(err, "failed to extract name", "object", obj)
		name = "unknown"
	}
	c.logger.V(2).Info("resource deleted", "name", name)
}
