package ttl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
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
	lock            sync.Mutex
	infoMetric      metric.Int64ObservableGauge
}

func NewManager(
	metadataInterface metadata.Interface,
	discoveryInterface discovery.DiscoveryInterface,
	checker checker.AuthChecker,
	timeInterval time.Duration,
) controllers.Controller {
	logger := logging.WithName(ControllerName)
	meterProvider := otel.GetMeterProvider()
	meter := meterProvider.Meter(metrics.MeterName)
	infoMetric, err := meter.Int64ObservableGauge(
		"kyverno_ttl_controller_info",
		metric.WithDescription("can be used to track individual resource controllers running for ttl based cleanup"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_ttl_controller_info")
	}
	mgr := &manager{
		metadataClient:  metadataInterface,
		discoveryClient: discoveryInterface,
		checker:         checker,
		resController:   map[schema.GroupVersionResource]stopFunc{},
		logger:          logger,
		interval:        timeInterval,
		infoMetric:      infoMetric,
	}
	if infoMetric != nil {
		if _, err := meter.RegisterCallback(mgr.report, infoMetric); err != nil {
			logger.Error(err, "failed to register callback")
		}
	}
	return mgr
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
	var informerWaitGroup wait.Group
	informerWaitGroup.StartWithContext(cont, func(ctx context.Context) {
		logger.V(3).Info("informer starting...")
		defer logger.V(3).Info("informer stopping...")
		informer.Informer().Run(cont.Done())
	})
	stopInformer := func() {
		// Send stop signal to informer's goroutine
		cancel()
		// Wait for the group to terminate
		informerWaitGroup.Wait()
	}
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stopInformer()
		return fmt.Errorf("failed to wait for cache sync: %s", gvr.Resource)
	}
	controller, err := newController(m.metadataClient.Resource(gvr), informer, logger, gvr)
	if err != nil {
		stopInformer()
		return err
	}
	var controllerWaitGroup wait.Group
	controllerWaitGroup.StartWithContext(cont, func(ctx context.Context) {
		logger.V(3).Info("controller starting...")
		defer logger.V(3).Info("controller stopping...")
		controller.Start(ctx, workers)
	})
	m.resController[gvr] = func() {
		stopInformer()
		controller.Stop()
		controllerWaitGroup.Wait()
	}
	return nil
}

func (m *manager) filterPermissionsResource(resources []schema.GroupVersionResource) []schema.GroupVersionResource {
	validResources := []schema.GroupVersionResource{}
	for _, resource := range resources {
		// Check if the service account has the necessary permissions
		if HasResourcePermissions(m.logger, resource, m.checker) {
			validResources = append(validResources, resource)
		}
	}
	return validResources
}

func (m *manager) report(ctx context.Context, observer metric.Observer) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for gvr := range m.resController {
		observer.ObserveInt64(
			m.infoMetric,
			1,
			metric.WithAttributes(
				attribute.String("resource_group", gvr.Group),
				attribute.String("resource_version", gvr.Version),
				attribute.String("resource_resource", gvr.Resource),
			),
		)
	}
	return nil
}

func (m *manager) reconcile(ctx context.Context, workers int) error {
	defer m.logger.V(3).Info("manager reconciliation done")
	m.logger.V(3).Info("beginning reconciliation", "interval", m.interval)
	desiredState, err := m.getDesiredState()
	if err != nil {
		return err
	}
	observedState, err := m.getObservedState()
	if err != nil {
		return err
	}
	m.lock.Lock()
	defer m.lock.Unlock()
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
