package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alitto/pond"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/imageverification"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/mutation"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
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
	urLister   kyvernov2listers.UpdateRequestNamespaceLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	urGenerator webhookgenerate.Generator
	eventGen    event.Interface
	pcBuilder   webhookutils.PolicyContextBuilder

	admissionReports             bool
	backgroundServiceAccountName string
	reportsServiceAccountName    string
	auditPool                    *pond.WorkerPool
	reportingConfig              reportutils.ReportingConfiguration
	breaker.Breaker
}

func NewHandlers(
	engine engineapi.Engine,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	pCache policycache.Cache,
	nsLister corev1listers.NamespaceLister,
	urLister kyvernov2listers.UpdateRequestNamespaceLister,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urGenerator webhookgenerate.Generator,
	eventGen event.Interface,
	admissionReports bool,
	backgroundServiceAccountName string,
	reportsServiceAccountName string,
	jp jmespath.Interface,
	maxAuditWorkers int,
	maxAuditCapacity int,
	reportingConfig reportutils.ReportingConfiguration,
) *resourceHandlers {
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
		reportsServiceAccountName:    reportsServiceAccountName,
		auditPool:                    pond.New(maxAuditWorkers, maxAuditCapacity, pond.Strategy(pond.Lazy())),
		reportingConfig:              reportingConfig,
	}
}

func (h *resourceHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind).WithValues("URLParams", request.URLParams)
	logger.V(4).Info("received an admission request in validating webhook")

	policies, mutatePolicies, generatePolicies, _, auditWarnPolicies, err := h.retrieveAndCategorizePolicies(ctx, logger, request, failurePolicy, false)
	if err != nil {
		return errorResponse(logger, request.UID, err, "failed to fetch policy with key")
	}

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 && len(auditWarnPolicies) == 0 {
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
		h.reportingConfig,
	)
	var wg wait.Group
	var ok bool
	var msg string
	var warnings []string
	var enforceResponses []engineapi.EngineResponse
	wg.Start(func() {
		ok, msg, warnings, enforceResponses = vh.HandleValidationEnforce(ctx, request, policies, auditWarnPolicies, startTime)
	})
	if !admissionutils.IsDryRun(request.AdmissionRequest) {
		var dummy wait.Group
		h.handleBackgroundApplies(ctx, logger, request, generatePolicies, mutatePolicies, startTime, &dummy)
	}
	wg.Wait()
	if !ok {
		logger.V(4).Info("admission request denied")
		events := webhookutils.GenerateEvents(enforceResponses, true, h.configuration)
		h.eventGen.Add(events...)
		return admissionutils.Response(request.UID, errors.New(msg), warnings...)
	}
	go h.auditPool.Submit(func() {
		auditResponses := vh.HandleValidationAudit(ctx, request)
		var events []event.Info

		switch {
		case len(auditResponses) == 0:
			events = webhookutils.GenerateEvents(enforceResponses, false, h.configuration)
		case len(enforceResponses) == 0:
			events = webhookutils.GenerateEvents(auditResponses, false, h.configuration)
		default:
			responses := mergeEngineResponses(auditResponses, enforceResponses)
			events = webhookutils.GenerateEvents(responses, false, h.configuration)
		}

		h.eventGen.Add(events...)
	})
	return admissionutils.ResponseSuccess(request.UID, warnings...)
}

func (h *resourceHandlers) Mutate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind).WithValues("URLParams", request.URLParams)
	logger.V(4).Info("received an admission request in mutating webhook")

	_, mutatePolicies, _, verifyImagesPolicies, _, err := h.retrieveAndCategorizePolicies(ctx, logger, request, failurePolicy, true) //nolint:dogsled
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
	mh := mutation.NewMutationHandler(logger, h.kyvernoClient, h.engine, h.eventGen, h.nsLister, h.metricsConfig, h.admissionReports, h.reportingConfig)
	patches, warnings, err := mh.HandleMutation(ctx, request, mutatePolicies, policyContext, startTime, h.configuration)
	if err != nil {
		logger.Error(err, "mutation failed")
		return admissionutils.Response(request.UID, err)
	}
	if len(verifyImagesPolicies) != 0 {
		newRequest := patchRequest(patches, request.AdmissionRequest, logger)
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
			h.reportingConfig,
		)
		imagePatches, imageVerifyWarnings, err := ivh.Handle(ctx, newRequest, verifyImagesPolicies, policyContext)
		if err != nil {
			logger.Error(err, "image verification failed")
			return admissionutils.Response(request.UID, err)
		}
		patches = jsonutils.JoinPatches(patches, imagePatches)
		warnings = append(warnings, imageVerifyWarnings...)
	}
	return admissionutils.MutationResponse(request.UID, patches, warnings...)
}

func (h *resourceHandlers) retrieveAndCategorizePolicies(
	ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, mutation bool) (
	[]kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, error,
) {
	var policies, mutatePolicies, generatePolicies, imageVerifyValidatePolicies, auditWarnPolicies []kyvernov1.PolicyInterface
	if request.URLParams == "" {
		gvr := schema.GroupVersionResource(request.Resource)
		policies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.ValidateEnforce, gvr, request.SubResource, request.Namespace)...)
		mutatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Mutate, gvr, request.SubResource, request.Namespace)...)
		generatePolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Generate, gvr, request.SubResource, request.Namespace)...)
		auditWarnPolicies = filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.ValidateAuditWarn, gvr, request.SubResource, request.Namespace)...)
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
			return nil, nil, nil, nil, nil, fmt.Errorf("key %s/%s: %v", polNamespace, polName, err)
		}

		filteredPolicies := filterPolicies(ctx, failurePolicy, policy)
		if len(filteredPolicies) == 0 {
			logger.V(4).Info("no policy found with key", "namespace", polNamespace, "name", polName)
			return nil, nil, nil, nil, nil, nil
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
		if spec.HasValidate() && *spec.EmitWarning {
			auditWarnPolicies = append(auditWarnPolicies, policy)
		}
	}
	return policies, mutatePolicies, generatePolicies, imageVerifyValidatePolicies, auditWarnPolicies, nil
}

func (h *resourceHandlers) buildPolicyContextFromAdmissionRequest(logger logr.Logger, request handlers.AdmissionRequest, policies []kyvernov1.PolicyInterface) (*policycontext.PolicyContext, error) {
	policyContext, err := h.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return nil, err
	}
	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels, err = engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, policies, logger)
		if err != nil {
			return nil, err
		}
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

func mergeEngineResponses(auditResponses, enforceResponses []engineapi.EngineResponse) []engineapi.EngineResponse {
	responseMap := make(map[string]engineapi.EngineResponse)
	var responses []engineapi.EngineResponse

	for _, enforceResponse := range enforceResponses {
		responseMap[enforceResponse.Policy().GetName()] = enforceResponse
	}

	for _, auditResponse := range auditResponses {
		policyName := auditResponse.Policy().GetName()
		if enforceResponse, exists := responseMap[policyName]; exists {
			response := auditResponse
			for _, ruleResponse := range enforceResponse.PolicyResponse.Rules {
				response.PolicyResponse.Add(ruleResponse.Stats(), ruleResponse)
			}
			responses = append(responses, response)
			delete(responseMap, policyName)
		} else {
			responses = append(responses, auditResponse)
		}
	}

	if len(responseMap) != 0 {
		for _, enforceResponse := range responseMap {
			responses = append(responses, enforceResponse)
		}
	}

	return responses
}
