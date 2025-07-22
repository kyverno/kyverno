package imageverification

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type ImageVerificationHandler interface {
	Handle(context.Context, admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext) ([]byte, []string, error)
}

type imageVerificationHandler struct {
	kyvernoClient    versioned.Interface
	engine           engineapi.Engine
	log              logr.Logger
	eventGen         event.Interface
	admissionReports bool
	cfg              config.Configuration
	nsLister         corev1listers.NamespaceLister
	reportConfig     reportutils.ReportingConfiguration
}

func NewImageVerificationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	eventGen event.Interface,
	admissionReports bool,
	cfg config.Configuration,
	nsLister corev1listers.NamespaceLister,
	reportConfig reportutils.ReportingConfiguration,
) ImageVerificationHandler {
	return &imageVerificationHandler{
		kyvernoClient:    kyvernoClient,
		engine:           engine,
		log:              log,
		eventGen:         eventGen,
		admissionReports: admissionReports,
		cfg:              cfg,
		nsLister:         nsLister,
		reportConfig:     reportConfig,
	}
}

func (h *imageVerificationHandler) Handle(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) ([]byte, []string, error) {
	ok, message, imagePatches, warnings := h.handleVerifyImages(ctx, h.log, request, policyContext, policies, h.cfg)
	if !ok {
		return nil, nil, errors.New(message)
	}
	h.log.V(6).Info("images verified", "patches", string(imagePatches), "warnings", warnings)
	return imagePatches, warnings, nil
}

func (h *imageVerificationHandler) handleVerifyImages(
	ctx context.Context,
	logger logr.Logger,
	request admissionv1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []kyvernov1.PolicyInterface,
	cfg config.Configuration,
) (bool, string, []byte, []string) {
	if len(policies) == 0 {
		return true, "", nil, nil
	}
	var engineResponses []engineapi.EngineResponse
	var patches []jsonpatch.JsonPatchOperation
	verifiedImageData := engineapi.ImageVerificationMetadata{}
	failurePolicy := kyvernov1.Ignore

	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				if policy.GetSpec().GetFailurePolicy(ctx) == kyvernov1.Fail {
					failurePolicy = kyvernov1.Fail
				}

				policyContext := policyContext.WithPolicy(policy)
				if request.Kind.Kind != "Namespace" && request.Namespace != "" {
					namespaceLabels, err := engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, []kyvernov1.PolicyInterface{policy}, h.log)
					if err != nil {
						h.log.Error(err, "failed to get namespace labels for policy", "policy", policy.GetName())
						return
					}
					policyContext = policyContext.WithNamespaceLabels(namespaceLabels)
				}

				resp, ivm := h.engine.VerifyAndPatchImages(ctx, policyContext)
				if !resp.IsEmpty() {
					engineResponses = append(engineResponses, resp)
				}

				patches = append(patches, resp.GetPatches()...)
				verifiedImageData.Merge(ivm)
			},
		)
	}

	blocked := webhookutils.BlockRequest(engineResponses, failurePolicy, logger)
	events := webhookutils.GenerateEvents(engineResponses, blocked, cfg)
	h.eventGen.Add(events...)

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, webhookutils.GetBlockedMessages(engineResponses), nil, nil
	}

	if !verifiedImageData.IsEmpty() {
		hasAnnotations := hasAnnotations(policyContext)
		annotationPatches, err := verifiedImageData.Patches(hasAnnotations, logger)
		if err != nil {
			logger.Error(err, "failed to create image verification annotation patches")
		} else {
			// add annotation patches first
			patches = append(annotationPatches, patches...)
		}
	}

	go h.handleAudit(ctx, policyContext.NewResource(), request, nil, engineResponses...)

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", jsonutils.JoinPatches(patch.ConvertPatches(patches...)...), warnings
}

func hasAnnotations(context *engine.PolicyContext) bool {
	newResource := context.NewResource()
	annotations := newResource.GetAnnotations()
	return len(annotations) != 0
}

func (v *imageVerificationHandler) handleAudit(
	ctx context.Context,
	resource unstructured.Unstructured,
	request admissionv1.AdmissionRequest,
	_ map[string]string,
	engineResponses ...engineapi.EngineResponse,
) {
	createReport := v.admissionReports
	if !v.reportConfig.ImageVerificationReportsEnabled() {
		createReport = false
	}
	if admissionutils.IsDryRun(request) {
		createReport = false
	}
	// we don't need reports for deletions and when it's about sub resources
	if request.Operation == admissionv1.Delete || request.SubResource != "" {
		createReport = false
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		createReport = false
	}
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			if createReport {
				report := reportutils.BuildAdmissionReport(resource, request, engineResponses...)
				if len(report.GetResults()) > 0 {
					err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
						_, err := reportutils.CreateEphemeralReport(context.Background(), report, v.kyvernoClient)
						return err
					})
					if err != nil {
						v.log.Error(err, "failed to create report")
					}
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
}
