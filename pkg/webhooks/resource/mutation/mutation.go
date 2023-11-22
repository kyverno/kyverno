package mutation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type MutationHandler interface {
	// HandleMutation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleMutation(context.Context, admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, time.Time) ([]byte, []string, error)
}

func NewMutationHandler(
	log logr.Logger,
	engine engineapi.Engine,
	eventGen event.Interface,
	nsLister corev1listers.NamespaceLister,
	metrics metrics.MetricsConfigManager,
) MutationHandler {
	return &mutationHandler{
		log:      log,
		engine:   engine,
		eventGen: eventGen,
		nsLister: nsLister,
		metrics:  metrics,
	}
}

type mutationHandler struct {
	log      logr.Logger
	engine   engineapi.Engine
	eventGen event.Interface
	nsLister corev1listers.NamespaceLister
	metrics  metrics.MetricsConfigManager
}

func (h *mutationHandler) HandleMutation(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) ([]byte, []string, error) {
	mutatePatches, mutateEngineResponses, err := h.applyMutations(ctx, request, policies, policyContext)
	if err != nil {
		return nil, nil, err
	}
	h.log.V(6).Info("", "generated patches", string(mutatePatches))
	return mutatePatches, webhookutils.GetWarningMessages(mutateEngineResponses), nil
}

// applyMutations handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (v *mutationHandler) applyMutations(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) ([]byte, []engineapi.EngineResponse, error) {
	if len(policies) == 0 {
		return nil, nil, nil
	}

	var patches []jsonpatch.JsonPatchOperation
	var engineResponses []engineapi.EngineResponse
	failurePolicy := kyvernov1.Ignore

	for _, policy := range policies {
		spec := policy.GetSpec()
		if !spec.HasMutate() {
			continue
		}

		err := tracing.ChildSpan1(
			ctx,
			"",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) error {
				v.log.V(3).Info("applying policy mutate rules", "policy", policy.GetName())
				currentContext := policyContext.WithPolicy(policy)
				if policy.GetSpec().GetFailurePolicy(ctx) == kyvernov1.Fail {
					failurePolicy = kyvernov1.Fail
				}

				engineResponse, policyPatches, err := v.applyMutation(ctx, request, currentContext, failurePolicy)
				if err != nil {
					return fmt.Errorf("mutation policy %s error: %v", policy.GetName(), err)
				}

				if len(policyPatches) > 0 {
					patches = append(patches, policyPatches...)
					rules := engineResponse.GetSuccessRules()
					if len(rules) != 0 {
						v.log.Info("mutation rules from policy applied successfully", "policy", policy.GetName(), "rules", rules)
					}
				}

				if engineResponse != nil {
					policyContext = currentContext.WithNewResource(engineResponse.PatchedResource)
					engineResponses = append(engineResponses, *engineResponse)
				}

				return nil
			},
		)
		if err != nil {
			return nil, nil, err
		}
	}

	events := webhookutils.GenerateEvents(engineResponses, false)
	v.eventGen.Add(events...)

	logMutationResponse(patches, engineResponses, v.log)

	// patches holds all the successful patches, if no patch is created, it returns nil
	return jsonutils.JoinPatches(patch.ConvertPatches(patches...)...), engineResponses, nil
}

func (h *mutationHandler) applyMutation(ctx context.Context, request admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, failurePolicy kyvernov1.FailurePolicyType) (*engineapi.EngineResponse, []jsonpatch.JsonPatchOperation, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		policyContext = policyContext.WithNamespaceLabels(engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log))
	}

	engineResponse := h.engine.Mutate(ctx, policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() {
		if webhookutils.BlockRequest([]engineapi.EngineResponse{engineResponse}, failurePolicy, h.log) {
			h.log.Info("failed to apply policy, blocking request", "policy", policyContext.Policy().GetName(), "rules", engineResponse.GetFailedRulesWithErrors())
			return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy().GetName(), engineResponse.GetFailedRulesWithErrors())
		} else {
			h.log.Info("ignoring unsuccessful engine responses", "policy", policyContext.Policy().GetName(), "rules", engineResponse.GetFailedRulesWithErrors())
			return &engineResponse, nil, nil
		}
	}

	return &engineResponse, policyPatches, nil
}

func logMutationResponse(patches []jsonpatch.JsonPatchOperation, engineResponses []engineapi.EngineResponse, logger logr.Logger) {
	if len(patches) != 0 {
		logger.V(4).Info("created patches", "count", len(patches))
	}

	// if any of the policies fails, print out the error
	if !engineutils.IsResponseSuccessful(engineResponses) {
		logger.Error(fmt.Errorf(webhookutils.GetErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
	}
}
