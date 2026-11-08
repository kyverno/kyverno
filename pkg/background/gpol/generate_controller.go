package gpol

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	event "github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	restmapperutils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// CELGenerateController is used to process URs that are generated as a result of an event from the trigger resource.
type CELGenerateController struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// mapper
	restMapper meta.RESTMapper

	context      libs.Context
	engine       gpolengine.Engine
	provider     gpolengine.Provider
	watchManager *WatchManager

	statusControl common.StatusControlInterface
	configuration config.Configuration

	breaker.Breaker

	eventGen event.Interface

	log logr.Logger
}

// NewCELGenerateController creates a new CELGenerateController.
func NewCELGenerateController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	context libs.Context,
	engine gpolengine.Engine,
	provider gpolengine.Provider,
	watchManager *WatchManager,
	statusControl common.StatusControlInterface,
	eventGen event.Interface,
	log logr.Logger,
	configuration config.Configuration,
) *CELGenerateController {
	restMapper, err := restmapperutils.GetRESTMapper(client)
	if err != nil || restMapper == nil {
		log.Error(err, "failed to get RESTMapper, falling back to default mapper")
		restMapper = meta.NewDefaultRESTMapper(nil)
	}
	return &CELGenerateController{
		client:        client,
		kyvernoClient: kyvernoClient,
		restMapper:    restMapper,
		context:       context,
		engine:        engine,
		provider:      provider,
		watchManager:  watchManager,
		statusControl: statusControl,
		eventGen:      eventGen,
		log:           log,
		configuration: configuration,
	}
}

func (c *CELGenerateController) ProcessUR(ur *kyvernov2.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey())
	generatedResources := make([]kyverno.ResourceSpec, 0)
	logger.V(2).Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	var failures []error
	// One cost budget for the whole update request, established outside the loop. Inside
	// it, every iteration would start from a fresh context and so get its own ceiling,
	// which bounds each rule context rather than the request that contains them.
	ctx, releaseBudget := requestbudget.EnsureBackground(context.TODO())
	defer releaseBudget()
	for i := 0; i < len(ur.Spec.RuleContext); i++ {
		if ur.Spec.RuleContext[i].DeleteDownstream {
			c.watchManager.DeleteDownstreams(ur.Spec.GetPolicyKey(), &ur.Spec.RuleContext[i].Trigger)
			continue
		}
		trigger, err := common.GetTrigger(c.client, ur.Spec, i, c.log)
		if err != nil || trigger == nil {
			logger.V(4).Info("the trigger resource does not exist or is pending creation")
			failures = append(failures, fmt.Errorf("gpol %s failed: failed to fetch trigger resource: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		if c.configuration != nil && c.configuration.ToFilter(trigger.GroupVersionKind(), "", trigger.GetNamespace(), trigger.GetName()) {
			logger.V(4).Info("trigger resource is filtered out by resource filters", "kind", trigger.GetKind(), "namespace", trigger.GetNamespace(), "name", trigger.GetName())
			continue
		}
		workerCtx := c.context.Clone()
		// Only this UpdateRequest's recorded UID-qualified downstreams may
		// authorize adoption of unmarked historical CEL resources. The
		// library still checks live UID and trigger labels before any write.
		if setter, ok := workerCtx.(interface{ SetAcknowledgedGeneratedResources([]kyverno.ResourceSpec) }); ok {
			setter.SetAcknowledgedGeneratedResources(ur.Status.GeneratedResources)
		}
		var request celengine.EngineRequest
		admissionRequest := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
		if admissionRequest == nil {
			gvk := trigger.GroupVersionKind()
			mapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return fmt.Errorf("failed to map gvk to gvr %s (%v)", gvk, err)
			}

			gvr := mapping.Resource
			request = celengine.Request(
				workerCtx,
				trigger.GroupVersionKind(),
				gvr,
				"",
				trigger.GetName(),
				trigger.GetNamespace(),
				admissionv1.Create,
				authenticationv1.UserInfo{},
				trigger,
				nil,
				false,
				nil,
			)
		} else {
			request = celengine.RequestFromAdmission(workerCtx, *admissionRequest)
		}
		policy, err := c.provider.Get(ctx, ur.Spec.GetPolicyKey())
		if err != nil {
			logger.Error(err, "failed to fetch gpol", "gpol", ur.Spec.GetPolicyKey())
			failures = append(failures, fmt.Errorf("gpol %s failed: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		isSync := policy.Policy.GetSpec().SynchronizationEnabled()
		// Covers engine.Handle's write and the SyncWatchers call below that
		// catches the watcher cache up; keyed like the labels the engine stamps.
		endGenerate := c.watchManager.BeginGenerate(ur.Spec.GetPolicyKey(), trigger.GetUID())
		gpolResponse, err := c.engine.Handle(ctx, request, policy, ur.Spec.RuleContext[i].CacheRestore)
		if err != nil {
			endGenerate()
			logger.Error(err, "failed to generate resources for gpol", "gpol", ur.Spec.GetPolicyKey())
			failures = append(failures, fmt.Errorf("gpol %s failed: %v", ur.Spec.GetPolicyKey(), err))
			continue
		}
		var reportableEngineResponses []engineapi.EngineResponse
		for _, res := range gpolResponse.Policies {
			if res.Result == nil {
				logger.V(4).Info("no resources generated by gpol", "gpol", ur.Spec.GetPolicyKey(), "policy", res.Policy.GetName())
				if ur.Spec.RuleContext[i].Synchronize {
					// the trigger was updated and no longer matches the policy
					// (match constraints or match conditions), delete the
					// downstream resources previously generated for it.
					logger.V(2).Info("trigger no longer matches the policy, deleting downstream resources", "gpol", ur.Spec.GetPolicyKey(), "trigger", ur.Spec.RuleContext[i].Trigger.String())
					c.watchManager.DeleteDownstreams(ur.Spec.GetPolicyKey(), &ur.Spec.RuleContext[i].Trigger)
				}
				continue
			}
			engineResponse := engineapi.EngineResponse{
				Resource:       *gpolResponse.Trigger,
				PolicyResponse: engineapi.PolicyResponse{},
			}
			engineResponse.PolicyResponse.Rules = []engineapi.RuleResponse{*res.Result}
			engineResponse = engineResponse.WithPolicy(engineapi.NewGeneratingPolicyFromLike(res.Policy))
			if res.Result.Status() == engineapi.RuleStatusSkip {
				c.eventGen.Add(event.NewPolicyExceptionEvents(engineResponse, *res.Result, event.GeneratePolicyController)...)
			} else if res.Result.Status() == engineapi.RuleStatusError || res.Result.Status() == engineapi.RuleStatusFail {
				// A failed evaluation can still contain acknowledged downstream writes.
				// Report them with the failed UR and register them additively, without
				// treating an incomplete set as a desired set for deletion.
				resourcesToTrack, trackingErr := c.retainAcknowledgedDownstreamsChecked(ctx, ur, ur.Spec.RuleContext[i].Trigger, res.Result.GeneratedResources())
				if trackingErr != nil {
					failures = append(failures, fmt.Errorf("gpol %s failed to verify prior downstream: %w", ur.Spec.GetPolicyKey(), trackingErr))
				}
				for _, resource := range resourcesToTrack {
					generatedResources = append(generatedResources, kyverno.ResourceSpec{
						Kind:       resource.GetKind(),
						APIVersion: resource.GetAPIVersion(),
						Name:       resource.GetName(),
						Namespace:  resource.GetNamespace(),
						UID:        resource.GetUID(),
					})
				}
				if isSync && len(resourcesToTrack) > 0 {
					if err := c.watchManager.AddWatchers(ur.Spec.GetPolicyKey(), &ur.Spec.RuleContext[i].Trigger, resourcesToTrack, policy.Policy.GetSpec().UseServerSideApply); err != nil {
						logger.Error(err, "failed to track partially generated resources", "gpol", ur.Spec.GetPolicyKey())
						failures = append(failures, fmt.Errorf("gpol %s failed to track acknowledged downstreams: %w", ur.Spec.GetPolicyKey(), err))
					}
				}
				// surface evaluation/generation errors instead of silently
				// reporting the UR as Completed, so users can diagnose why no
				// resources were generated (https://github.com/kyverno/kyverno/issues/16983)
				err := fmt.Errorf("gpol %s rule %s %s for trigger %s: %s", ur.Spec.GetPolicyKey(), res.Result.Name(), res.Result.Status(), ur.Spec.RuleContext[i].Trigger.String(), res.Result.Message())
				logger.Error(err, "failed to generate resources for gpol", "gpol", ur.Spec.GetPolicyKey(), "rule", res.Result.Name(), "status", res.Result.Status())
				failures = append(failures, err)
			} else {
				resourcesToSync := res.Result.GeneratedResources()
				for _, resource := range resourcesToSync {
					generatedResources = append(generatedResources, kyverno.ResourceSpec{
						Kind:       resource.GetKind(),
						APIVersion: resource.GetAPIVersion(),
						Name:       resource.GetName(),
						Namespace:  resource.GetNamespace(),
						UID:        resource.GetUID(),
					})
				}

				if res.Result.Status() == engineapi.RuleStatusPass &&
					isSync &&
					(!ur.Spec.RuleContext[i].CacheRestore || len(resourcesToSync) > 0) {
					// Synchronous so the watcher cache holds this write's hash before
					// ProcessUR returns.
					watchersSynced := true
					if len(resourcesToSync) > 0 {
						if err := c.watchManager.SyncWatchers(ur.Spec.GetPolicyKey(), &ur.Spec.RuleContext[i].Trigger, resourcesToSync, policy.Policy.GetSpec().UseServerSideApply); err != nil {
							logger.Error(err, "failed to sync watchers for generated resources", "gpol", ur.Spec.GetPolicyKey())
							watchersSynced = false
							failures = append(failures, fmt.Errorf("gpol %s failed to sync downstream watchers: %w", ur.Spec.GetPolicyKey(), err))
						} else {
							logger.V(4).Info("synced watchers for generated resources", "gpol", ur.Spec.GetPolicyKey())
						}
					}
					if ur.Spec.RuleContext[i].Synchronize && watchersSynced {
						// Trigger updates reconcile the watcher cache. CREATE retries
						// must use the UID-verified cleanup below instead; a stale cache
						// entry could otherwise delete a same-name replacement.
						c.watchManager.CleanupStaleDownstreams(ur.Spec.GetPolicyKey(), &ur.Spec.RuleContext[i].Trigger, resourcesToSync)
					}
				}
				// A fully successful restore can generate zero objects. In that
				// case skip broad cache reconciliation above, but still clean
				// acknowledged obsolete objects using verified UID preconditions.
				if res.Result.Status() == engineapi.RuleStatusPass && isSync && len(ur.Status.GeneratedResources) > 0 {
					pending, err := c.cleanupAcknowledgedAfterRetry(ctx, ur.Spec.GetPolicyKey(), ur.Spec.RuleContext[i].Trigger, ur.Status.GeneratedResources, resourcesToSync)
					generatedResources = append(generatedResources, pending...)
					if err != nil {
						failures = append(failures, fmt.Errorf("gpol %s stale downstream cleanup failed: %w", ur.Spec.GetPolicyKey(), err))
					}
				}
			}
			if err := c.audit(context.TODO(), engineResponse, generatedResources); err != nil {
				logger.Error(err, "failed to audit gpol", "gpol", ur.Spec.GetPolicyKey())
			}
			if len(engineResponse.PolicyResponse.Rules) > 0 &&
				reportutils.IsPolicyReportable(engineapi.NewGeneratingPolicyFromLike(res.Policy)) {
				reportableEngineResponses = append(reportableEngineResponses, engineResponse)
			}
		}
		endGenerate()
		if c.needsReports(*trigger) && len(reportableEngineResponses) > 0 {
			if err := c.createReports(context.TODO(), *trigger, reportableEngineResponses...); err != nil {
				c.log.Error(err, "failed to create report")
			}
		}
	}
	if len(failures) > 0 {
		// A transient read outage must not erase the only durable record of a
		// previously acknowledged write. Preserve its UID in failed status,
		// but never register an unverified object for synchronization/cleanup.
		generatedResources = c.preserveUnverifiedFailedStatus(ctx, ur.Status.GeneratedResources, generatedResources)
	}
	return updateURStatus(c.statusControl, *ur, multierr.Combine(failures...), generatedResources)
}

// matchesAcknowledgedPolicy allows safe recovery of CEL downstreams generated
// before the explicit scope label existed. A legacy object is accepted only
// when the other policy scope has no same-name policy; an ambiguous or failed
// API lookup never authorizes adoption or deletion.
func (c *CELGenerateController) matchesAcknowledgedPolicy(ctx context.Context, labels map[string]string, policyKey string) (bool, error) {
	if matchesPolicyKey(labels, policyKey) {
		return true, nil
	}
	if _, marked := labels[common.GeneratePolicyScopeLabel]; marked {
		return false, nil
	}
	policyNamespace, policyName, err := cache.SplitMetaNamespaceKey(policyKey)
	if err != nil || labels[common.GeneratePolicyLabel] != policyName {
		return false, nil
	}
	if policyNamespace != "" && !common.MatchesPolicyTriggerNamespace(labels, policyNamespace) {
		return false, nil
	}
	counterpartKind := "GeneratingPolicy"
	counterpartNamespace := ""
	if policyNamespace == "" {
		counterpartKind = "NamespacedGeneratingPolicy"
		counterpartNamespace = labels[common.GenerateTriggerNSLabel]
		if counterpartNamespace == "" && labels[common.GenerateTriggerKindLabel] == "Namespace" {
			counterpartNamespace = labels[common.GenerateTriggerNameLabel]
		}
		if counterpartNamespace == "" {
			return false, nil
		}
	}
	_, err = c.client.GetResource(ctx, "policies.kyverno.io/v1", counterpartKind, counterpartNamespace, policyName)
	if apierrors.IsNotFound(err) {
		// Only callers with a UID-qualified identity from the persisted
		// UpdateRequest may use this legacy fallback. Absence of a competing
		// policy alone is not ownership proof.
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("verify legacy counterpart %s %s/%s: %w", counterpartKind, counterpartNamespace, policyName, err)
	}
	return false, nil
}

// retainAcknowledgedDownstreams preserves the identities recorded by prior
// failed attempts. A retry or cache restore can error before evaluating the
// expression that originally wrote a downstream, but the live object still
// needs synchronization and policy-deletion tracking. Never adopt objects
// which no longer have the expected Kyverno policy/trigger ownership.
// retainAcknowledgedDownstreams is used by existing regression fixtures;
// processing uses the checked variant to surface transient ownership errors.
func (c *CELGenerateController) retainAcknowledgedDownstreams(ctx context.Context, ur *kyvernov2.UpdateRequest, trigger kyverno.ResourceSpec, current []*unstructured.Unstructured) []*unstructured.Unstructured {
	resources, _ := c.retainAcknowledgedDownstreamsChecked(ctx, ur, trigger, current)
	return resources
}

func (c *CELGenerateController) retainAcknowledgedDownstreamsChecked(ctx context.Context, ur *kyvernov2.UpdateRequest, trigger kyverno.ResourceSpec, current []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	if len(ur.Status.GeneratedResources) == 0 {
		return current, nil
	}
	key := func(apiVersion, kind, namespace, name string) string {
		group := apiVersion
		if gv, err := schema.ParseGroupVersion(apiVersion); err == nil {
			group = gv.Group
		}
		return group + "\x00" + kind + "\x00" + namespace + "\x00" + name
	}
	seen := make(map[string]struct{}, len(current)+len(ur.Status.GeneratedResources))
	for _, resource := range current {
		seen[key(resource.GetAPIVersion(), resource.GetKind(), resource.GetNamespace(), resource.GetName())] = struct{}{}
	}
	for _, spec := range ur.Status.GeneratedResources {
		if spec.APIVersion == "" || spec.Kind == "" || spec.Name == "" || spec.UID == "" {
			continue
		}
		identity := key(spec.APIVersion, spec.Kind, spec.Namespace, spec.Name)
		if _, alreadyTracked := seen[identity]; alreadyTracked {
			continue
		}
		resource, err := c.client.GetResource(ctx, spec.APIVersion, spec.Kind, spec.Namespace, spec.Name)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return current, fmt.Errorf("read acknowledged downstream %s/%s: %w", spec.Namespace, spec.Name, err)
		}
		if resource == nil || resource.GetUID() != spec.UID {
			continue // never adopt a same-name replacement
		}
		labels := resource.GetLabels()
		if labels[kyverno.LabelAppManagedBy] != kyverno.ValueKyvernoApp ||
			labels[common.GenerateTriggerUIDLabel] != string(trigger.UID) {
			continue
		}
		owned, ownershipErr := c.matchesAcknowledgedPolicy(ctx, labels, ur.Spec.GetPolicyKey())
		if ownershipErr != nil {
			return current, ownershipErr
		}
		if !owned {
			continue
		}
		current = append(current, resource)
		seen[identity] = struct{}{}
	}
	return current, nil
}

// preserveUnverifiedFailedStatus keeps UID-qualified previous identities only
// when a transient Kubernetes read error prevents verification. A confirmed
// NotFound or a readable replacement is not carried forward. This affects the
// failed UR status only; no unverified object enters the WatchManager cache.
func (c *CELGenerateController) preserveUnverifiedFailedStatus(ctx context.Context, previous, current []kyverno.ResourceSpec) []kyverno.ResourceSpec {
	key := func(spec kyverno.ResourceSpec) string {
		group := spec.APIVersion
		if gv, err := schema.ParseGroupVersion(spec.APIVersion); err == nil {
			group = gv.Group
		}
		return group + "\x00" + spec.Kind + "\x00" + spec.Namespace + "\x00" + spec.Name
	}
	seen := make(map[string]struct{}, len(current)+len(previous))
	for _, spec := range current {
		seen[key(spec)] = struct{}{}
	}
	for _, spec := range previous {
		if spec.APIVersion == "" || spec.Kind == "" || spec.Name == "" || spec.UID == "" {
			continue
		}
		identity := key(spec)
		if _, ok := seen[identity]; ok {
			continue
		}
		resource, err := c.client.GetResource(ctx, spec.APIVersion, spec.Kind, spec.Namespace, spec.Name)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil || (resource != nil && resource.GetUID() == spec.UID) {
			// Keep the durable record, not the unverified live object. A
			// readable replacement with a different UID must never be adopted.
			current = append(current, spec)
			seen[identity] = struct{}{}
		}
	}
	return current
}

// cleanupAcknowledgedAfterRetry handles stale partial writes recorded by the
// failed UpdateRequest even after watcher caches have been lost in a restart.
// Only a live object with the original UID and verified policy/trigger ownership
// may be deleted. DeleteOptions carries a UID precondition to guard races.
// Failed API operations retain their original identity for the next retry.
func (c *CELGenerateController) cleanupAcknowledgedAfterRetry(
	ctx context.Context, policyKey string, trigger kyverno.ResourceSpec,
	previous []kyverno.ResourceSpec, desired []*unstructured.Unstructured,
) ([]kyverno.ResourceSpec, error) {
	key := func(apiVersion, kind, namespace, name string) string {
		group := apiVersion
		if gv, err := schema.ParseGroupVersion(apiVersion); err == nil {
			group = gv.Group
		}
		return group + "\x00" + kind + "\x00" + namespace + "\x00" + name
	}
	wanted := make(map[string]struct{}, len(desired))
	for _, res := range desired {
		wanted[key(res.GetAPIVersion(), res.GetKind(), res.GetNamespace(), res.GetName())] = struct{}{}
	}
	seen := make(map[string]struct{}, len(previous))
	var pending []kyverno.ResourceSpec
	var failures []error
	for _, spec := range previous {
		if spec.APIVersion == "" || spec.Kind == "" || spec.Name == "" || spec.UID == "" {
			continue
		}
		identity := key(spec.APIVersion, spec.Kind, spec.Namespace, spec.Name)
		if _, exists := wanted[identity]; exists {
			continue
		}
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		live, err := c.client.GetResource(ctx, spec.APIVersion, spec.Kind, spec.Namespace, spec.Name)
		if apierrors.IsNotFound(err) {
			if c.watchManager != nil {
				c.watchManager.ForgetDownstreamUID(policyKey, spec.UID)
			}
			continue
		}
		if err != nil {
			pending = append(pending, spec)
			failures = append(failures, fmt.Errorf("read stale downstream %s/%s: %w", spec.Namespace, spec.Name, err))
			continue
		}
		if live == nil || live.GetUID() != spec.UID {
			continue
		}
		labels := live.GetLabels()
		if labels[kyverno.LabelAppManagedBy] != kyverno.ValueKyvernoApp ||
			labels[common.GenerateTriggerUIDLabel] != string(trigger.UID) {
			continue
		}
		owned, ownershipErr := c.matchesAcknowledgedPolicy(ctx, labels, policyKey)
		if ownershipErr != nil {
			pending = append(pending, spec)
			failures = append(failures, ownershipErr)
			continue
		}
		if !owned {
			continue
		}
		uid := spec.UID
		// Mark stale before issuing DELETE: an asynchronous delete event must
		// not recreate this now-obsolete object from a still-active cache.
		if c.watchManager != nil {
			c.watchManager.SuppressDownstreamRecreation(policyKey, uid)
		}
		err = c.client.DeleteResource(ctx, spec.APIVersion, spec.Kind, spec.Namespace, spec.Name, false,
			metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) {
			pending = append(pending, spec)
			failures = append(failures, fmt.Errorf("delete stale downstream %s/%s: %w", spec.Namespace, spec.Name, err))
		} else if c.watchManager != nil {
			c.watchManager.ForgetDownstreamUID(policyKey, uid)
		}
	}
	return pending, multierr.Combine(failures...)
}

func (c *CELGenerateController) audit(ctx context.Context, engineResponse engineapi.EngineResponse, generatedResources []kyverno.ResourceSpec) error {
	if engineResponse.IsEmpty() {
		return nil
	}

	// Check if there are any rule results before accessing Rules[0]
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		return nil
	}

	if engineResponse.IsSuccessful() {
		c.eventGen.Add(event.NewBackgroundSuccessEvent(event.GeneratePolicyController, engineResponse.Policy(), generatedResources)...)
		for _, gen := range generatedResources {
			c.eventGen.Add(event.NewResourceGenerationEvent(
				engineResponse.Policy().GetName(),
				engineResponse.PolicyResponse.Rules[0].Name(),
				event.GeneratePolicyController,
				gen))
		}
	}
	if engineResponse.IsFailed() || engineResponse.IsError() {
		trigger := kyverno.ResourceSpec{
			Kind:       engineResponse.Resource.GetKind(),
			APIVersion: engineResponse.Resource.GetAPIVersion(),
			Name:       engineResponse.Resource.GetName(),
			Namespace:  engineResponse.Resource.GetNamespace(),
		}
		c.eventGen.Add(event.NewBackgroundFailedEvent(
			errors.New(engineResponse.PolicyResponse.Rules[0].Message()),
			engineResponse.Policy(),
			engineResponse.PolicyResponse.Rules[0].Name(),
			event.GeneratePolicyController,
			trigger)...)
	}

	return nil
}

func (c *CELGenerateController) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	engineResponses ...engineapi.EngineResponse,
) error {
	if resource.GetName() == "" || resource.GetUID() == "" {
		c.log.V(3).Info("skipping report creation for subresource with empty name or uid", "gvk", resource.GroupVersionKind())
		return nil
	}

	report := reportutils.BuildGenerateReport(resource.GetNamespace(), resource.GroupVersionKind(), resource.GetName(), resource.GetUID(), engineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, c.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CELGenerateController) needsReports(trigger unstructured.Unstructured) bool {
	createReport := reportutils.ReportingCfg.GenerateReportsEnabled()
	if !reportutils.IsGvkSupported(trigger.GroupVersionKind()) {
		createReport = false
	}
	return createReport
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error, genResources []kyverno.ResourceSpec) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}
