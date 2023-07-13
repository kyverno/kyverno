package ttlcontroller

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"github.com/kyverno/kyverno/pkg/config"
)

type stopFunc = context.CancelFunc

type manager struct {
	metadataClient  metadata.Interface
	discoveryClient discovery.DiscoveryInterface
	checker         checker.AuthChecker
	resController   map[schema.GroupVersionResource]stopFunc
}

func NewManager(metadataInterface metadata.Interface, discoveryInterface discovery.DiscoveryInterface,  authorizationInterface authorizationv1client.AuthorizationV1Interface) (*manager, error) {

	selfChecker := checker.NewSelfChecker(authorizationInterface.SelfSubjectAccessReviews())

	resController := make(map[schema.GroupVersionResource]stopFunc)

	return &manager{
		metadataClient:  metadataInterface,
		discoveryClient: discoveryInterface,
		checker:         selfChecker,
		resController:   resController,
	}, nil
}

func (m *manager) Run(ctx context.Context) error {
	defer func() {
		// Stop all informers and wait for them to finish
		for gvr := range m.resController {
			if err := m.stop(ctx, gvr); err != nil {
				log.Println("Error stopping informer:", err)
			}
		}
	}()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := m.reconcile(ctx); err != nil {
				return err
			}
		}
	}
}

func (m *manager) getDesiredState() (sets.Set[schema.GroupVersionResource], error) {
	// Get the list of resources currently present in the cluster
	newresources, err := discoverResources(m.discoveryClient)
	if err != nil {
		return nil, err
	}
	validResources := m.filterPermissionsResource(newresources)
	return sets.New(validResources...), nil
}

func (m *manager) getObservedState() (sets.Set[schema.GroupVersionResource], error) {
	observedState := sets.New[schema.GroupVersionResource]()
	for resource := range m.resController {
		observedState.Insert(resource)
	}
	return observedState, nil
}

func (m *manager) stop(ctx context.Context, gvr schema.GroupVersionResource) error {
	if stopFunc, ok := m.resController[gvr]; ok {
		delete(m.resController, gvr)
		func() {
			// defer log.Println("stopped", gvr)
			log.Println("stopping...", gvr)
			stopFunc()
		}()
	}
	return nil
}

func (m *manager) start(ctx context.Context, gvr schema.GroupVersionResource) error {
	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
	options := func(options *metav1.ListOptions) {
		options.LabelSelector = config.CleanupLabel
	}

	informer := metadatainformer.NewFilteredMetadataInformer(m.metadataClient,
		gvr,
		metav1.NamespaceAll,
		10*time.Minute,
		indexers,
		options,
	)

	controller := newController(m.metadataClient.Resource(gvr), informer)

	cont, cancel := context.WithCancel(ctx)
	var wg wait.Group

	stopFunc := func() {
		cancel() // Send stop signal to informer's goroutine
		wg.Wait()     // Wait for the group to terminate
		controller.Stop()
		log.Println("Stopped", gvr)
	}


	wg.StartWithContext(cont, func(ctx context.Context){
		log.Println("informer starting...", gvr)
		informer.Informer().Run(cont.Done())
	})


	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		cancel()
		return fmt.Errorf("failed to wait for cache sync: %s", gvr)
	}


	log.Println("controller starting...", gvr)
	controller.Start(cont, 3)
	m.resController[gvr] = stopFunc // Store the stop function
	return nil
}

func (m *manager) reconcile(ctx context.Context) error {
	log.Println("start manager reconciliation")
	desiredState, err := m.getDesiredState()
	if err != nil {
		return err
	}
	observedState, err := m.getObservedState()
	if err != nil {
		return err
	}
	for gvr := range observedState.Difference(desiredState) {
		if err := m.stop(ctx, gvr); err != nil {
			return err
		}
	}
	for gvr := range desiredState.Difference(observedState) {
		if err := m.start(ctx, gvr); err != nil {
			return err
		}
	}
	return nil
}
