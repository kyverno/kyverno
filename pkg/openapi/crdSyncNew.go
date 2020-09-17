package openapi

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	prun "k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/log"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/util/wait"
	crdInformer "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	crdLister "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type crdSyncNew struct {
	client     *client.Client
	controller *Controller
	crdLister crdLister.CustomResourceDefinitionLister
	crdListerSynced cache.InformerSynced
	queue    workqueue.RateLimitingInterface
}

func NewCRDSyncNew(client *client.Client, controller *Controller, crdInformer crdInformer.CustomResourceDefinitionInformer) *crdSyncNew {
	if controller == nil {
		panic(fmt.Errorf("nil controller sent into crd sync"))
	}

	cs := &crdSyncNew{
		controller: controller,
		client:     client,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "crds"),
	}

	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// controller.ParseCRD(unstructured.Unstructured{Object: obj.(map[string]interface{})})
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				cs.queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				cs.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			cs.controller.deleteCRDFromCache(obj.(*crdv1.CustomResourceDefinition).Spec.Names.Kind)
		},
	})

	cs.crdLister = crdInformer.Lister()
	cs.crdListerSynced = crdInformer.Informer().HasSynced
	stopCh := make(chan struct{})
	go crdInformer.Informer().Run(stopCh)
	return cs
}

func (c *crdSyncNew) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncToStdout(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *crdSyncNew) syncToStdout(key string) error {
	fmt.Println("Key:", key)
	res, err := c.crdLister.Get(key)
	if err != nil {
		return err
	}
	unstructuredObj, err := prun.DefaultUnstructuredConverter.ToUnstructured(res)
	if err != nil{
		return err
	}
	c.controller.ParseCRD(unstructured.Unstructured{Object:unstructuredObj})
	return nil
}

func (c *crdSyncNew) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 5 {
		log.Log.Info(fmt.Sprintf("Error syncing CRD %v: %v", key, err))

		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	runtime.HandleError(err)
	log.Log.Info(fmt.Sprintf("Dropping CRD %q out of the queue: %v", key, err))
}

func (c *crdSyncNew) Run(threadiness int, stopCh <-chan struct{}) {
	newDoc, err := c.client.DiscoveryClient.OpenAPISchema()
	if err != nil {
		log.Log.Error(err, "cannot get OpenAPI schema")
	}

	err = c.controller.useOpenApiDocument(newDoc)
	if err != nil {
		log.Log.Error(err, "Could not set custom OpenAPI document")
	}
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	log.Log.Info("Starting CRD controller")

	if !cache.WaitForCacheSync(stopCh, c.crdListerSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	log.Log.Info("Stopping CRD controller")
}

func (c *crdSyncNew) runWorker() {
	for c.processNextItem() {
	}
}