package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alitto/pond"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/d4f"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/imageverification"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/mutation"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type resourceHandlers struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	engine        engineapi.Engine

	// config
	configuration config.Configuration
	metricsConfig metrics.MetricsConfigManager

	// cache
	pCache policycache.Cache

	// listers
	nsLister   corev1listers.NamespaceLister
	urLister   kyvernov1beta1listers.UpdateRequestNamespaceLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	urGenerator webhookgenerate.Generator
	eventGen    event.Interface
	pcBuilder   webhookutils.PolicyContextBuilder

	admissionReports             bool
	backgroundServiceAccountName string
	auditPool                    *pond.WorkerPool
	reportsBreaker               d4f.Breaker
}

func NewHandlers(
	engine engineapi.Engine,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	pCache policycache.Cache,
	nsLister corev1listers.NamespaceLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urGenerator webhookgenerate.Generator,
	eventGen event.Interface,
	admissionReports bool,
	backgroundServiceAccountName string,
	jp jmespath.Interface,
	maxAuditWorkers int,
	maxAuditCapacity int,
	reportsBreaker d4f.Breaker,
) webhooks.ResourceHandlers {
	return &resourceHandlers{
		engine:                       engine,
		client:                       client,
		kyvernoClient:                kyvernoClient,
		configuration:                configuration,
		metricsConfig:                metricsConfig,
		pCache:                       pCache,
		nsLister:                     nsLister,
		urLister:                     urLister,
		cpolLister:                   cpolInformer.Lister(),
		polLister:                    polInformer.Lister(),
		urGenerator:                  urGenerator,
		eventGen:                     eventGen,
		pcBuilder:                    webhookutils.NewPolicyContextBuilder(configuration, jp),
		admissionReports:             admissionReports,
		backgroundServiceAccountName: backgroundServiceAccountName,
		auditPool:                    pond.New(maxAuditWorkers, maxAuditCapacity, pond.Strategy(pond.Lazy())),
		reportsBreaker:               reportsBreaker,
	}
}

func (h *resourceHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind).WithValues("URLParams", request.URLParams)
	logger.V(4).Info("received an admission request in validating webhook")

	policies, mutatePolicies, generatePolicies, _, err := h.retrieveAndCategorizePolicies(ctx, logger, request, failurePolicy, false)
	if err != nil {
		return errorResponse(logger, request.UID, err, "failed to fetch policy with key")
	}

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 {
		logger.V(4).Info("no policies matched admission request")
	}

	logger.V(4).Info("processing policies for validate admission request", "validate", len(policies), "mutate", len(mutatePolicies), "generate", len(generatePolicies))

	vh := validation.NewValidationHandler(
		logger,
		h.kyvernoClient,
		h.engine,
		h.pCache,
		h.pcBuilder,
		h.eventGen,
		h.admissionReports,
		h.metricsConfig,
		h.configuration,
		h.nsLister,
		h.reportsBreaker,
	)
	var wg sync.WaitGroup
	var ok bool
	var msg string
	var warnings []string
	wg.Add(1)
	go func() {
		defer wg.Done()
		ok, msg, warnings = vh.HandleValidationEnforce(ctx, request, policies, startTime)
	}()

	go h.auditPool.Submit(func() {
		vh.HandleValidationAudit(ctx, request)
	})
	if !admissionutils.IsDryRun(request.AdmissionRequest) {
		h.handleBackgroundApplies(ctx, logger, request, generatePolicies, mutatePolicies, startTime, nil)
	}
	if len(policies) == 0 {
		return admissionutils.ResponseSuccess(request.UID)
	}

	wg.Wait()
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.Response(request.UID, errors.New(msg), warnings...)
	}

	return admissionutils.ResponseSuccess(request.UID, warnings...)
}

func (h *resourceHandlers) Mutate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind).WithValues("URLParams", request.URLParams)
	logger.V(4).Info("received an admission request in mutating webhook")

	_, mutatePolicies, _, verifyImagesPolicies, err := h.retrieveAndCategorizePolicies(ctx, logger, request, failurePolicy, true)
	if err != nil {
		return errorResponse(logger, request.UID, err, "failed to fetch policy with key")
	}
	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched mutate admission request")
		return admissionutils.ResponseSuccess(request.UID)
	}
	logger.V(4).Info("processing policies for mutate admission request", "mutatePolicies", len(mutatePolicies), "verifyImagesPolicies", len(verifyImagesPolicies))
	policyContext, err := h.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.Response(request.UID, err)
	}
	mh := mutation.NewMutationHandler(logger, h.engine, h.eventGen, h.nsLister, h.metricsConfig)
	mutatePatches, mutateWarnings, err := mh.HandleMutation(ctx, request.AdmissionRequest, mutatePolicies, policyContext, startTime)
	if err != nil {
		logger.Error(err, "mutation failed")
		return admissionutils.Response(request.UID, err)
	}
	newRequest := patchRequest(mutatePatches, request.AdmissionRequest, logger)
	// rebuild context to process images updated via mutate policies
	policyContext, err = h.pcBuilder.Build(newRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.Response(request.UID, err)
	}
	ivh := imageverification.NewImageVerificationHandler(
		logger,
		h.kyvernoClient,
		h.engine,
		h.eventGen,
		h.admissionReports,
		h.configuration,
		h.nsLister,
		h.reportsBreaker,
	)
	imagePatches, imageVerifyWarnings, err := ivh.Handle(ctx, newRequest, verifyImagesPolicies, policyContext)
	if err != nil {
		logger.Error(err, "image verification failed")
		return admissionutils.Response(request.UID, err)
	}
	patch := jsonutils.JoinPatches(mutatePatches, imagePatches)
	var warnings []string
	warnings = append(warnings, mutateWarnings...)
	warnings = append(warnings, imageVerifyWarnings...)
	return admissionutils.MutationResponse(request.UID, patch, warnings...)
}

func (h *resourceHandlers) retrieveAndCategorizePolicies(
	ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, mutation bool) (
	[]kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, error,
) {
	var policies, mutatePolicies, generatePolicies, imageVerifyValidatePolicies []kyvernov1.PolicyInterface
	if request.URLParams == "" {
		gvr := schema.GroupVersionResource(request.Resource)
		policies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.ValidateEnforce, gvr, request.SubResource, request.Namespace)...)
		mutatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Mutate, gvr, request.SubResource, request.Namespace)...)
		generatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Generate, gvr, request.SubResource, request.Namespace)...)
		if mutation {
			imageVerifyValidatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesMutate, gvr, request.SubResource, request.Namespace)...)
		} else {
			imageVerifyValidatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesValidate, gvr, request.SubResource, request.Namespace)...)
			policies = append(policies, imageVerifyValidatePolicies...)
		}
	} else {
		meta := strings.Split(request.URLParams, "/")
		polName := meta[1]
		polNamespace := ""

		if len(meta) >= 3 {
			polNamespace = meta[1]
			polName = meta[2]
		}

		var policy kyvernov1.PolicyInterface
		var err error
		if polNamespace == "" {
			policy, err = h.cpolLister.Get(polName)
		} else {
			policy, err = h.polLister.Policies(polNamespace).Get(polName)
		}
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("key %s/%s: %v", polNamespace, polName, err)
		}

		filteredPolicies := filterPolicies(ctx, failurePolicy, policy)
		if len(filteredPolicies) == 0 {
			logger.V(4).Info("no policy found with key", "namespace", polNamespace, "name", polName)
			return nil, nil, nil, nil, nil
		}
		policy = filteredPolicies[0]
		spec := policy.GetSpec()
		if spec.HasValidate() {
			policies = append(policies, policy)
		}
		if spec.HasGenerate() {
			generatePolicies = append(generatePolicies, policy)
		}
		if spec.HasMutate() {
			mutatePolicies = append(mutatePolicies, policy)
		}
		if spec.HasVerifyImages() {
			policies = append(policies, policy)
		}
	}
	return policies, mutatePolicies, generatePolicies, imageVerifyValidatePolicies, nil
}

func (h *resourceHandlers) buildPolicyContextFromAdmissionRequest(logger logr.Logger, request handlers.AdmissionRequest) (*policycontext.PolicyContext, error) {
	policyContext, err := h.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return nil, err
	}
	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}
	policyContext = policyContext.WithNamespaceLabels(namespaceLabels)
	return policyContext, nil
}

func filterPolicies(ctx context.Context, failurePolicy string, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var results []kyvernov1.PolicyInterface
	for _, policy := range policies {
		if failurePolicy == "fail" {
			if policy.GetSpec().GetFailurePolicy(ctx) == kyvernov1.Fail {
				results = append(results, policy)
			}
		} else if failurePolicy == "ignore" {
			if policy.GetSpec().GetFailurePolicy(ctx) == kyvernov1.Ignore {
				results = append(results, policy)
			}
		} else {
			results = append(results, policy)
		}
	}
	return results
}
