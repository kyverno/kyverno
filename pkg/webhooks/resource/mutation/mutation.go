package mutation

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/utils"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type MutationHandler interface {
	// HandleMutation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleMutation(
		*metrics.MetricsConfig,
		*admissionv1.AdmissionRequest,
		[]kyvernov1.PolicyInterface,
		*engine.PolicyContext,
		// map[string]string,
		time.Time,
	) ([]byte, []string, error)
}

func NewMutationHandler(
	log logr.Logger,
	eventGen event.Interface,
	openApiManager openapi.ValidateInterface,
	nsLister corev1listers.NamespaceLister,
) MutationHandler {
	return &mutationHandler{
		log:            log,
		eventGen:       eventGen,
		openApiManager: openApiManager,
		nsLister:       nsLister,
	}
}

type mutationHandler struct {
	log            logr.Logger
	eventGen       event.Interface
	openApiManager openapi.ValidateInterface
	nsLister       corev1listers.NamespaceLister
}

func (h *mutationHandler) HandleMutation(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) ([]byte, []string, error) {
	mutatePatches, mutateEngineResponses, err := h.applyMutations(metricsConfig, request, policies, policyContext)
	if err != nil {
		return nil, nil, err
	}
	h.log.V(6).Info("", "generated patches", string(mutatePatches))
	return mutatePatches, webhookutils.GetWarningMessages(mutateEngineResponses), nil
}

// applyMutations handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (v *mutationHandler) applyMutations(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) ([]byte, []*response.EngineResponse, error) {
	if len(policies) == 0 {
		return nil, nil, nil
	}

	if isResourceDeleted(policyContext) && request.Operation == admissionv1.Update {
		return nil, nil, nil
	}

	var patches [][]byte
	var engineResponses []*response.EngineResponse

	for _, policy := range policies {
		spec := policy.GetSpec()
		if !spec.HasMutate() {
			continue
		}
		v.log.V(3).Info("applying policy mutate rules", "policy", policy.GetName())
		policyContext.Policy = policy
		engineResponse, policyPatches, err := v.applyMutation(request, policyContext)
		if err != nil {
			return nil, nil, fmt.Errorf("mutation policy %s error: %v", policy.GetName(), err)
		}

		if len(policyPatches) > 0 {
			patches = append(patches, policyPatches...)
			rules := engineResponse.GetSuccessRules()
			if len(rules) != 0 {
				v.log.Info("mutation rules from policy applied successfully", "policy", policy.GetName(), "rules", rules)
			}
		}

		policyContext.NewResource = engineResponse.PatchedResource
		engineResponses = append(engineResponses, engineResponse)

		// registering the kyverno_policy_results_total metric concurrently
		go webhookutils.RegisterPolicyResultsMetricMutation(v.log, metricsConfig, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go webhookutils.RegisterPolicyExecutionDurationMetricMutate(v.log, metricsConfig, string(request.Operation), policy, *engineResponse)
	}

	// generate annotations
	if annPatches := utils.GenerateAnnotationPatches(engineResponses, v.log); annPatches != nil {
		patches = append(patches, annPatches...)
	}

	if !isResourceDeleted(policyContext) {
		events := webhookutils.GenerateEvents(engineResponses, false)
		v.eventGen.Add(events...)
	}

	logMutationResponse(patches, engineResponses, v.log)

	// patches holds all the successful patches, if no patch is created, it returns nil
	return jsonutils.JoinPatches(patches...), engineResponses, nil
}

func (h *mutationHandler) applyMutation(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext) (*response.EngineResponse, [][]byte, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log)
	}

	engineResponse := engine.Mutate(policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() {
		return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy.GetName(), engineResponse.GetFailedRules())
	}

	if policyContext.Policy.ValidateSchema() && engineResponse.PatchedResource.GetKind() != "*" {
		err := h.openApiManager.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetAPIVersion(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to validate resource mutated by policy %s", policyContext.Policy.GetName())
		}
	}

	return engineResponse, policyPatches, nil
}

func logMutationResponse(patches [][]byte, engineResponses []*response.EngineResponse, logger logr.Logger) {
	if len(patches) != 0 {
		logger.V(4).Info("created patches", "count", len(patches))
	}

	// if any of the policies fails, print out the error
	if !engineutils.IsResponseSuccessful(engineResponses) {
		logger.Error(errors.New(webhookutils.GetErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
	}
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
