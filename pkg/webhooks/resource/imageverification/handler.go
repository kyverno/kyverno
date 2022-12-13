package imageverification

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ImageVerificationHandler interface {
	Handle(context.Context, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext) ([]byte, []string, error)
}

type imageVerificationHandler struct {
	kyvernoClient    versioned.Interface
	rclient          registryclient.Client
	log              logr.Logger
	eventGen         event.Interface
	admissionReports bool
}

func NewImageVerificationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	rclient registryclient.Client,
	eventGen event.Interface,
	admissionReports bool,
) ImageVerificationHandler {
	return &imageVerificationHandler{
		kyvernoClient:    kyvernoClient,
		rclient:          rclient,
		log:              log,
		eventGen:         eventGen,
		admissionReports: admissionReports,
	}
}

func (h *imageVerificationHandler) Handle(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) ([]byte, []string, error) {
	ok, message, imagePatches, warnings := h.handleVerifyImages(ctx, h.log, request, policyContext, policies)
	if !ok {
		return nil, nil, errors.New(message)
	}
	h.log.V(6).Info("images verified", "patches", string(imagePatches), "warnings", warnings)
	return imagePatches, warnings, nil
}

func (h *imageVerificationHandler) handleVerifyImages(
	ctx context.Context,
	logger logr.Logger,
	request *admissionv1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []kyvernov1.PolicyInterface,
) (bool, string, []byte, []string) {
	if len(policies) == 0 {
		return true, "", nil, nil
	}
	var engineResponses []*response.EngineResponse
	var patches [][]byte
	verifiedImageData := &engine.ImageVerificationMetadata{}
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)
				resp, ivm := engine.VerifyAndPatchImages(ctx, h.rclient, policyContext)

				engineResponses = append(engineResponses, resp)
				patches = append(patches, resp.GetPatches()...)
				verifiedImageData.Merge(ivm)
			},
		)
	}

	failurePolicy := policies[0].GetSpec().GetFailurePolicy()
	blocked := webhookutils.BlockRequest(engineResponses, failurePolicy, logger)
	if !isResourceDeleted(policyContext) {
		events := webhookutils.GenerateEvents(engineResponses, blocked)
		h.eventGen.Add(events...)
	}

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
	return true, "", jsonutils.JoinPatches(patches...), warnings
}

func hasAnnotations(context *engine.PolicyContext) bool {
	newResource := context.NewResource()
	annotations := newResource.GetAnnotations()
	return len(annotations) != 0
}

func isResourceDeleted(policyContext *engine.PolicyContext) bool {
	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		resource := policyContext.NewResource()
		deletionTimeStamp = resource.GetDeletionTimestamp()
	} else {
		resource := policyContext.OldResource()
		deletionTimeStamp = resource.GetDeletionTimestamp()
	}
	return deletionTimeStamp != nil
}

func (v *imageVerificationHandler) handleAudit(
	ctx context.Context,
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
	namespaceLabels map[string]string,
	engineResponses ...*response.EngineResponse,
) {
	if !v.admissionReports {
		return
	}
	if request.DryRun != nil && *request.DryRun {
		return
	}
	// we don't need reports for deletions and when it's about sub resources
	if request.Operation == admissionv1.Delete || request.SubResource != "" {
		return
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		return
	}
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			report := reportutils.BuildAdmissionReport(resource, request, request.Kind, engineResponses...)
			// if it's not a creation, the resource already exists, we can set the owner
			if request.Operation != admissionv1.Create {
				gv := metav1.GroupVersion{Group: request.Kind.Group, Version: request.Kind.Version}
				controllerutils.SetOwner(report, gv.String(), request.Kind.Kind, resource.GetName(), resource.GetUID())
			}
			if len(report.GetResults()) > 0 {
				_, err := reportutils.CreateReport(context.Background(), report, v.kyvernoClient)
				if err != nil {
					v.log.Error(err, "failed to create report")
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
}
