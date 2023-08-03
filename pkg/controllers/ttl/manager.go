package ttl

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

type stopFunc = context.CancelFunc

const (
	Workers        = 3
	ControllerName = "ttl-controller-manager"
)

type manager struct {
	metadataClient  metadata.Interface
	discoveryClient discovery.DiscoveryInterface
	checker         checker.AuthChecker
	resController   map[schema.GroupVersionResource]stopFunc
	logger          logr.Logger
	interval        time.Duration
}

func NewManager(
	metadataInterface metadata.Interface,
	discoveryInterface discovery.DiscoveryInterface,
	authorizationInterface authorizationv1client.AuthorizationV1Interface,
	timeInterval time.Duration,
) controllers.Controller {
	logger := logging.WithName(ControllerName)
	selfChecker := checker.NewSelfChecker(authorizationInterface.SelfSubjectAccessReviews())
	resController := map[schema.GroupVersionResource]stopFunc{}
	return &manager{
		metadataClient:  metadataInterface,
		discoveryClient: discoveryInterface,
		checker:         selfChecker,
		resController:   resController,
		logger:          logger,
		interval:        timeInterval,
	}
}

func (m *manager) Run(ctx context.Context, worker int) {
	defer func() {
		// Stop all informers and wait for them to finish
		for gvr := range m.resController {
			logger := m.logger.WithValues("gvr", gvr)
			if err := m.stop(ctx, gvr); err != nil {
				logger.Error(err, "failed to stop informer")
			}
		}
	}()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.reconcile(ctx, worker); err != nil {
				m.logger.Error(err, "reconciliation failed")
				return
			}
		}
	}
}

func (m *manager) getDesiredState() (sets.Set[schema.GroupVersionResource], error) {
	// Get the list of resources currently present in the cluster
	newresources, err := discoverResources(m.logger, m.discoveryClient)
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
	logger := m.logger.WithValues("gvr", gvr)
	if stopFunc, ok := m.resController[gvr]; ok {
		delete(m.resController, gvr)
		func() {
			defer logger.Info("controller stopped")
			logger.Info("stopping controller...")
			stopFunc()
		}()
	}
	return nil
}

func (m *manager) start(ctx context.Context, gvr schema.GroupVersionResource, workers int) error {
	logger := m.logger.WithValues("gvr", gvr)
	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
	options := func(options *metav1.ListOptions) {
		options.LabelSelector = kyverno.LabelCleanupTtl
	}
	informer := metadatainformer.NewFilteredMetadataInformer(m.metadataClient,
		gvr,
		metav1.NamespaceAll,
		10*time.Minute,
		indexers,
		options,
	)
	cont, cancel := context.WithCancel(ctx)
	var wg wait.Group
	wg.StartWithContext(cont, func(ctx context.Context) {
		logger.Info("informer starting...")
		informer.Informer().Run(cont.Done())
	})
	stopInformer := func() {
		// Send stop signal to informer's goroutine
		cancel()
		// Wait for the group to terminate
		wg.Wait()
	}
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stopInformer()
		return fmt.Errorf("failed to wait for cache sync: %s", gvr.Resource)
	}
	controller, err := newController(m.metadataClient.Resource(gvr), informer, logger)
	if err != nil {
		stopInformer()
		return err
	}
	logger.Info("controller starting...")
	controller.Start(cont, workers)
	m.resController[gvr] = func() {
		stopInformer()
		controller.Stop()
	}
	return nil
}

func (m *manager) filterPermissionsResource(resources []schema.GroupVersionResource) []schema.GroupVersionResource {
	validResources := []schema.GroupVersionResource{}
	for _, resource := range resources {
		// Check if the service account has the necessary permissions
		if hasResourcePermissions(m.logger, resource, m.checker) {
			validResources = append(validResources, resource)
		}
	}
	return validResources
}

func (m *manager) reconcile(ctx context.Context, workers int) error {
	defer m.logger.Info("manager reconciliation done")
	m.logger.Info("start manager reconciliation")
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
		if err := m.start(ctx, gvr, workers); err != nil {
			return err
		}
	}
	return nil
}
