package imageverification

import (
	"errors"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageVerificationHandler interface {
	Handle(
		*metrics.MetricsConfig,
		*admissionv1.AdmissionRequest,
		[]kyvernov1.PolicyInterface,
		*engine.PolicyContext,
	) ([]byte, []string, error)
}

func NewImageVerificationHandler(log logr.Logger, eventGen event.Interface) ImageVerificationHandler {
	return &imageVerificationHandler{
		log:      log,
		eventGen: eventGen,
	}
}

type imageVerificationHandler struct {
	log      logr.Logger
	eventGen event.Interface
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
