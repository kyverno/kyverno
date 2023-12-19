package resource

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
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
	}
}

func (h *resourceHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in validating webhook")

	// timestamp at which this admission request got triggered
	gvr := schema.GroupVersionResource(request.Resource)
	policies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.ValidateEnforce, gvr, request.SubResource, request.Namespace)...)
	mutatePolicies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Mutate, gvr, request.SubResource, request.Namespace)...)
	generatePolicies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Generate, gvr, request.SubResource, request.Namespace)...)
	imageVerifyValidatePolicies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesValidate, gvr, request.SubResource, request.Namespace)...)
	policies = append(policies, imageVerifyValidatePolicies...)

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 {
		logger.V(4).Info("no policies matched admission request")
	}

	logger.V(4).Info("processing policies for validate admission request", "validate", len(policies), "mutate", len(mutatePolicies), "generate", len(generatePolicies))

	policyContext, err := h.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return errorResponse(logger, request.UID, err, "failed create policy context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}
	policyContext = policyContext.WithNamespaceLabels(namespaceLabels)
	vh := validation.NewValidationHandler(logger, h.kyvernoClient, h.engine, h.pCache, h.pcBuilder, h.eventGen, h.admissionReports, h.metricsConfig, h.configuration)

	ok, msg, warnings := vh.HandleValidation(ctx, request, policies, policyContext, startTime)
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.Response(request.UID, errors.New(msg), warnings...)
	}
	if !admissionutils.IsDryRun(request.AdmissionRequest) {
		go h.handleBackgroundApplies(ctx, logger, request.AdmissionRequest, policyContext, generatePolicies, mutatePolicies, startTime)
	}
	return admissionutils.ResponseSuccess(request.UID, warnings...)
}

func (h *resourceHandlers) Mutate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in mutating webhook")
	gvr := schema.GroupVersionResource(request.Resource)
	mutatePolicies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.Mutate, gvr, request.SubResource, request.Namespace)...)
	verifyImagesPolicies := filterPolicies(ctx, failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesMutate, gvr, request.SubResource, request.Namespace)...)
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
	ivh := imageverification.NewImageVerificationHandler(logger, h.kyvernoClient, h.engine, h.eventGen, h.admissionReports, h.configuration, h.nsLister)
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
