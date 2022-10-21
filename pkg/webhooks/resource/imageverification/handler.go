package imageverification

import (
	"context"
	"errors"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ImageVerificationHandler interface {
	Handle(
		*metrics.MetricsConfig,
		*admissionv1.AdmissionRequest,
		[]kyvernov1.PolicyInterface,
		*engine.PolicyContext,
	) ([]byte, []string, error)
}

func NewImageVerificationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	eventGen event.Interface,
	admissionReports bool,
) ImageVerificationHandler {
	return &imageVerificationHandler{
		kyvernoClient:    kyvernoClient,
		log:              log,
		eventGen:         eventGen,
		admissionReports: admissionReports,
	}
}

type imageVerificationHandler struct {
	kyvernoClient    versioned.Interface
	log              logr.Logger
	eventGen         event.Interface
	admissionReports bool
}

func (h *imageVerificationHandler) Handle(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) ([]byte, []string, error) {
	ok, message, imagePatches, warnings := h.handleVerifyImages(h.log, request, policyContext, policies)
	if !ok {
		return nil, nil, errors.New(message)
	}
	h.log.V(6).Info("images verified", "patches", string(imagePatches), "warnings", warnings)
	return imagePatches, warnings, nil
}

func (h *imageVerificationHandler) handleVerifyImages(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyvernov1.PolicyInterface) (bool, string, []byte, []string) {
	if len(policies) == 0 {
		return true, "", nil, nil
	}

	var engineResponses []*response.EngineResponse
	var patches [][]byte
	verifiedImageData := &engine.ImageVerificationMetadata{}
	for _, p := range policies {
		policyContext.Policy = p
		resp, ivm := engine.VerifyAndPatchImages(policyContext)

		engineResponses = append(engineResponses, resp)
		patches = append(patches, resp.GetPatches()...)
		verifiedImageData.Merge(ivm)
	}

	failurePolicy := policyContext.Policy.GetSpec().GetFailurePolicy()
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

	go h.handleAudit(policyContext.NewResource, request, nil, engineResponses...)

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", jsonutils.JoinPatches(patches...), warnings
}

func hasAnnotations(context *engine.PolicyContext) bool {
	annotations := context.NewResource.GetAnnotations()
	return len(annotations) != 0
}

func isResourceDeleted(policyContext *engine.PolicyContext) bool {
	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		deletionTimeStamp = policyContext.NewResource.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = policyContext.OldResource.GetDeletionTimestamp()
	}
	return deletionTimeStamp != nil
}

func (v *imageVerificationHandler) handleAudit(
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
	report := reportutils.NewAdmissionReport(resource, request, request.Kind, engineResponses...)
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
}
