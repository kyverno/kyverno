package resource

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metaclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	restmapper "github.com/kyverno/kyverno/pkg/utils/restmapper"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1informers "k8s.io/client-go/informers/admissionregistration/v1beta1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
	"k8s.io/client-go/tools/cache"
	watchTools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "resource-report-controller"
	maxRetries     = 5
)

type Resource struct {
	Namespace string
	Name      string
	Hash      string
}

type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Stopped  EventType = "STOPPED"
)

type EventHandler func(EventType, types.UID, schema.GroupVersionKind, Resource)

type MetadataCache interface {
	GetResourceHash(uid types.UID) (Resource, schema.GroupVersionKind, schema.GroupVersionResource, bool)
	GetAllResourceKeys() []string
	UpdateResourceHash(schema.GroupVersionResource, types.UID, Resource)
	AddEventHandler(EventHandler)
	Warmup(ctx context.Context) error
}

type Controller interface {
	controllers.Controller
	MetadataCache
}

type watcher struct {
	watcher watch.Interface
	gvk     schema.GroupVersionKind
	hashes  map[types.UID]Resource
}

type controller struct {
	// clients
	client dclient.Interface

	// listers
	polLister      kyvernov1listers.PolicyLister
	cpolLister     kyvernov1listers.ClusterPolicyLister
	vpolLister     policiesv1beta1listers.ValidatingPolicyLister
	nvpolLister    policiesv1beta1listers.NamespacedValidatingPolicyLister
	mpolLister     policiesv1beta1listers.MutatingPolicyLister
	nmpolLister    policiesv1beta1listers.NamespacedMutatingPolicyLister
	ivpolLister    policiesv1beta1listers.ImageValidatingPolicyLister
	nivpolLister   policiesv1beta1listers.NamespacedImageValidatingPolicyLister
	vapLister      admissionregistrationv1listers.ValidatingAdmissionPolicyLister
	mapLister      admissionregistrationv1beta1listers.MutatingAdmissionPolicyLister
	mapAlphaLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister
	metaClient     metaclient.UpstreamInterface

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	lock            sync.RWMutex
	dynamicWatchers map[schema.GroupVersionResource]*watcher
	eventHandlers   []EventHandler
}

func NewController(
	client dclient.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vpolInformer policiesv1beta1informers.ValidatingPolicyInformer,
	nvpolInformer policiesv1beta1informers.NamespacedValidatingPolicyInformer,
	mpolInformer policiesv1beta1informers.MutatingPolicyInformer,
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer,
	ivpolInformer policiesv1beta1informers.ImageValidatingPolicyInformer,
	nivpolInformer policiesv1beta1informers.NamespacedImageValidatingPolicyInformer,
	vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer,
	mapInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyInformer,
	mapAlphaInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer,
	metaClient metaclient.UpstreamInterface,
) Controller {
	c := controller{
		client:     client,
		polLister:  polInformer.Lister(),
		cpolLister: cpolInformer.Lister(),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
		),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		metaClient:      metaClient,
	}
	if vpolInformer != nil {
		c.vpolLister = vpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, vpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nvpolInformer != nil {
		c.nvpolLister = nvpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, nvpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mpolInformer != nil {
		c.mpolLister = mpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, mpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nmpolInformer != nil {
		c.nmpolLister = nmpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, nmpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if ivpolInformer != nil {
		c.ivpolLister = ivpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, ivpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nivpolInformer != nil {
		c.nivpolLister = nivpolInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, nivpolInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if vapInformer != nil {
		c.vapLister = vapInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, vapInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mapInformer != nil {
		c.mapLister = mapInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, mapInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mapAlphaInformer != nil {
		c.mapAlphaLister = mapAlphaInformer.Lister()
		if _, _, err := controllerutils.AddDefaultEventHandlers(logger, mapAlphaInformer.Informer(), c.queue); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if _, _, err := controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) Warmup(ctx context.Context) error {
	return c.updateDynamicWatchers(ctx)
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
	c.stopDynamicWatchers()
}

func (c *controller) GetResourceHash(uid types.UID) (Resource, schema.GroupVersionKind, schema.GroupVersionResource, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for gvr, watcher := range c.dynamicWatchers {
		if resource, exists := watcher.hashes[uid]; exists {
			return resource, watcher.gvk, gvr, true
		}
	}
	return Resource{}, schema.GroupVersionKind{}, schema.GroupVersionResource{}, false
}

func (c *controller) GetAllResourceKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var keys []string
	for _, watcher := range c.dynamicWatchers {
		for uid, resource := range watcher.hashes {
			key := string(uid)
			if resource.Namespace != "" {
				key = resource.Namespace + "/" + key
			}
			keys = append(keys, key)
		}
	}
	return keys
}

func (c *controller) UpdateResourceHash(gvr schema.GroupVersionResource, uid types.UID, hash Resource) {
	c.lock.Lock()
	defer c.lock.Unlock()
	w := c.dynamicWatchers[gvr]
	if w == nil {
		// watcher may have been removed by updateDynamicWatchers during concurrent policy changes
		return
	}
	w.hashes[uid] = hash
}

func (c *controller) AddEventHandler(eventHandler EventHandler) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler)
	for _, watcher := range c.dynamicWatchers {
		for uid, resource := range watcher.hashes {
			eventHandler(Added, uid, watcher.gvk, resource)
		}
	}
}

func (c *controller) startWatcher(ctx context.Context, logger logr.Logger, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) (*watcher, error) {
	hashes := map[types.UID]Resource{}
	var resourceVersion string

	w, ok := c.dynamicWatchers[gvr]
	// if we never started a watcher for this resource before, list the resources initially
	if !ok {
		objs, err := c.client.GetDynamicInterface().Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Error(err, "failed to list resources")
			return nil, err
		}
		resourceVersion = objs.GetResourceVersion()
		for _, obj := range objs.Items {
			uid := obj.GetUID()
			hash := reportutils.CalculateResourceHash(obj)
			hashes[uid] = Resource{
				Hash:      hash,
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}
		}
		// we started watcher for this resource before, use the previously existing hashes
	} else {
		// fetch the metadata to get the resource version
		metadata, err := c.metaClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		resourceVersion = metadata.GetResourceVersion()
		hashes = w.hashes
	}
	for uid := range hashes {
		c.notify(Added, uid, gvk, hashes[uid])
	}

	logger = logger.WithValues("resourceVersion", resourceVersion)
	logger.V(2).Info("start watcher ...")
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		logger.V(3).Info("creating watcher...")
		watch, err := c.client.GetDynamicInterface().Resource(gvr).Watch(context.Background(), options)
		if err != nil {
			logger.Error(err, "failed to watch")
		}
		return watch, err
	}
	watchInterface, err := watchTools.NewRetryWatcherWithContext(context.TODO(), resourceVersion, &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		logger.Error(err, "failed to create watcher")
		return nil, err
	}
	w = &watcher{
		watcher: watchInterface,
		gvk:     gvk,
		hashes:  hashes,
	}
	go func(gvr schema.GroupVersionResource) {
		defer logger.V(2).Info("watcher stopped")
		for event := range watchInterface.ResultChan() {
			switch event.Type {
			case watch.Added:
				c.updateHash(Added, event.Object.(*unstructured.Unstructured), gvr)
			case watch.Modified:
				c.updateHash(Modified, event.Object.(*unstructured.Unstructured), gvr)
			case watch.Deleted:
				c.deleteHash(event.Object.(*unstructured.Unstructured), gvr)
			case watch.Error:
				logger.Error(errors.New("watch error event received"), "watch error event received", "event", event.Object)
			}
		}
	}(gvr)
	return w, nil
}

func (c *controller) updateDynamicWatchers(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	clusterPolicies, err := utils.FetchClusterPolicies(c.cpolLister)
	if err != nil {
		return err
	}
	policies, err := utils.FetchPolicies(c.polLister, metav1.NamespaceAll)
	if err != nil {
		return err
	}
	kinds := utils.BuildKindSet(logger, utils.RemoveNonValidationPolicies(append(clusterPolicies, policies...)...)...)
	gvkToGvr := map[schema.GroupVersionKind]schema.GroupVersionResource{}
	for _, policyKind := range sets.List(kinds) {
		group, version, kind, subresource := kubeutils.ParseKindSelector(policyKind)
		c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
	}
	restMapper, err := restmapper.GetRESTMapper(c.client)
	if err != nil {
		return err
	}
	if c.vapLister != nil {
		vapPolicies, err := utils.FetchValidatingAdmissionPolicies(c.vapLister)
		if err != nil {
			return err
		}
		// fetch kinds from validating admission policies
		for _, policy := range vapPolicies {
			kinds := admissionpolicy.GetKinds(policy.Spec.MatchConstraints, restMapper)
			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.mapLister != nil {
		mapPolicies, err := utils.FetchMutatingAdmissionPolicies(c.mapLister)
		if err != nil {
			return err
		}
		for _, policy := range mapPolicies {
			converted := admissionpolicy.ConvertMatchResources(policy.Spec.MatchConstraints)
			kinds := admissionpolicy.GetKinds(converted, restMapper)
			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.mapAlphaLister != nil {
		mapAlphaPolicies, err := utils.FetchMutatingAdmissionPoliciesAlpha(c.mapAlphaLister)
		if err != nil {
			return err
		}
		for _, policy := range mapAlphaPolicies {
			var matchConstraints *admissionregistrationv1beta1.MatchResources
			if policy.Spec.MatchConstraints != nil {
				matchConstraints = &admissionregistrationv1beta1.MatchResources{
					ResourceRules:        convertResourceRulesForResourceController(policy.Spec.MatchConstraints.ResourceRules),
					ExcludeResourceRules: convertResourceRulesForResourceController(policy.Spec.MatchConstraints.ExcludeResourceRules),
					MatchPolicy:          (*admissionregistrationv1beta1.MatchPolicyType)(policy.Spec.MatchConstraints.MatchPolicy),
				}
			}
			converted := admissionpolicy.ConvertMatchResources(matchConstraints)
			kinds := admissionpolicy.GetKinds(converted, restMapper)
			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.vpolLister != nil {
		vpols, err := utils.FetchValidatingPolicies(c.vpolLister)
		if err != nil {
			return err
		}
		// fetch kinds from validating admission policies
		for _, policy := range vpols {
			kinds := admissionpolicy.GetKinds(policy.Spec.MatchConstraints, restMapper)
			for _, autogen := range policy.Status.Autogen.Configs {
				genKinds := admissionpolicy.GetKinds(autogen.Spec.MatchConstraints, restMapper)
				kinds = append(kinds, genKinds...)
			}

			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.nvpolLister != nil {
		vpols, err := utils.FetchNamespacedValidatingPolicies(c.nvpolLister, "")
		if err != nil {
			return err
		}
		// fetch kinds from validating admission policies
		for _, policy := range vpols {
			kinds := admissionpolicy.GetKinds(policy.Spec.MatchConstraints, restMapper)
			for _, autogen := range policy.Status.Autogen.Configs {
				genKinds := admissionpolicy.GetKinds(autogen.Spec.MatchConstraints, restMapper)
				kinds = append(kinds, genKinds...)
			}

			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.mpolLister != nil {
		mpols, err := utils.FetchMutatingPolicies(c.mpolLister)
		if err != nil {
			return err
		}
		for _, policy := range mpols {
			matchConstraints := policy.Spec.MatchConstraints
			kinds := admissionpolicy.GetKinds(matchConstraints, restMapper)
			for _, policy := range policy.Status.Autogen.Configs {
				matchConstraints := policy.Spec.MatchConstraints
				genKinds := admissionpolicy.GetKinds(matchConstraints, restMapper)

				kinds = append(kinds, genKinds...)
			}

			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.nmpolLister != nil {
		mpols, err := utils.FetchNamespacedMutatingPolicies(c.nmpolLister, "")
		if err != nil {
			return err
		}
		for _, policy := range mpols {
			matchConstraints := policy.Spec.MatchConstraints
			kinds := admissionpolicy.GetKinds(matchConstraints, restMapper)
			for _, policy := range policy.Status.Autogen.Configs {
				matchConstraints := policy.Spec.MatchConstraints
				genKinds := admissionpolicy.GetKinds(matchConstraints, restMapper)

				kinds = append(kinds, genKinds...)
			}

			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.ivpolLister != nil {
		ivpols, err := utils.FetchImageVerificationPolicies(c.ivpolLister)
		if err != nil {
			return err
		}
		// fetch kinds from image verification admission policies
		for _, policy := range ivpols {
			kinds := admissionpolicy.GetKinds(policy.Spec.MatchConstraints, restMapper)
			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	if c.nivpolLister != nil {
		ivpols, err := utils.FetchNamespacedImageVerificationPolicies(c.nivpolLister, "")
		if err != nil {
			return err
		}
		// fetch kinds from image verification admission policies
		for _, policy := range ivpols {
			kinds := admissionpolicy.GetKinds(policy.Spec.MatchConstraints, restMapper)
			for _, kind := range kinds {
				group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
				c.addGVKToGVRMapping(group, version, kind, subresource, gvkToGvr)
			}
		}
	}
	dynamicWatchers := map[schema.GroupVersionResource]*watcher{}
	for gvk, gvr := range gvkToGvr {
		logger := logger.WithValues("gvr", gvr, "gvk", gvk)
		// if we already have one, transfer it to the new map
		if c.dynamicWatchers[gvr] != nil {
			dynamicWatchers[gvr] = c.dynamicWatchers[gvr]
			delete(c.dynamicWatchers, gvr)
		} else {
			if w, err := c.startWatcher(ctx, logger, gvr, gvk); err != nil {
				logger.Error(err, "failed to start watcher")
			} else {
				dynamicWatchers[gvr] = w
			}
		}
	}
	oldDynamicWatcher := c.dynamicWatchers
	c.dynamicWatchers = dynamicWatchers
	// shutdown remaining watcher
	for gvr, watcher := range oldDynamicWatcher {
		watcher.watcher.Stop()
		delete(oldDynamicWatcher, gvr)
		for uid, resource := range watcher.hashes {
			c.notify(Stopped, uid, watcher.gvk, resource)
		}
	}
	return nil
}

func (c *controller) addGVKToGVRMapping(group, version, kind, subresource string, gvrMap map[schema.GroupVersionKind]schema.GroupVersionResource) {
	gvrss, err := c.client.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		logger.Error(err, "failed to get gvr from kind", "kind", kind)
	} else {
		for gvrs, api := range gvrss {
			if gvrs.SubResource == "" {
				gvk := schema.GroupVersionKind{Group: gvrs.Group, Version: gvrs.Version, Kind: gvrs.Kind}
				if !reportutils.IsGvkSupported(gvk) {
					logger.V(2).Info("kind is not supported", "gvk", gvk)
				} else {
					if slices.Contains(api.Verbs, "list") && slices.Contains(api.Verbs, "watch") {
						gvrMap[gvk] = gvrs.GroupVersionResource()
					} else {
						logger.V(2).Info("list/watch not supported for kind", "kind", kind)
					}
				}
			}
		}
	}
}

func (c *controller) stopDynamicWatchers() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, watcher := range c.dynamicWatchers {
		watcher.watcher.Stop()
	}
	c.dynamicWatchers = map[schema.GroupVersionResource]*watcher{}
}

func (c *controller) notify(eventType EventType, uid types.UID, gvk schema.GroupVersionKind, obj Resource) {
	for _, handler := range c.eventHandlers {
		handler(eventType, uid, gvk, obj)
	}
}

func (c *controller) updateHash(eventType EventType, obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	c.lock.Lock()
	defer c.lock.Unlock()
	watcher, exists := c.dynamicWatchers[gvr]
	if exists {
		uid := obj.GetUID()
		hash := reportutils.CalculateResourceHash(*obj)
		if exists && hash != watcher.hashes[uid].Hash {
			watcher.hashes[uid] = Resource{
				Hash:      hash,
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			}
			c.notify(eventType, uid, watcher.gvk, watcher.hashes[uid])
		}
	}
}

func (c *controller) deleteHash(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	c.lock.Lock()
	defer c.lock.Unlock()
	watcher, exists := c.dynamicWatchers[gvr]
	if exists {
		uid := obj.GetUID()
		hash := watcher.hashes[uid]
		delete(watcher.hashes, uid)
		c.notify(Deleted, uid, watcher.gvk, hash)
	}
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	return c.updateDynamicWatchers(ctx)
}

func convertResourceRulesForResourceController(rules []admissionregistrationv1alpha1.NamedRuleWithOperations) []admissionregistrationv1beta1.NamedRuleWithOperations {
	result := make([]admissionregistrationv1beta1.NamedRuleWithOperations, len(rules))
	for i, r := range rules {
		result[i] = admissionregistrationv1beta1.NamedRuleWithOperations{
			ResourceNames: r.ResourceNames,
			RuleWithOperations: admissionregistrationv1beta1.RuleWithOperations{
				Operations: convertOperationsForResourceController(r.Operations),
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   r.APIGroups,
					APIVersions: r.APIVersions,
					Resources:   r.Resources,
					Scope:       r.Scope,
				},
			},
		}
	}
	return result
}

func convertOperationsForResourceController(ops []admissionregistrationv1alpha1.OperationType) []admissionregistrationv1beta1.OperationType {
	result := make([]admissionregistrationv1beta1.OperationType, len(ops))
	copy(result, ops)
	return result
}
