package mutation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/tracing"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type MutationHandler interface {
	// HandleMutation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleMutation(context.Context, handlers.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, time.Time, config.Configuration) ([]byte, []string, error)
}

func NewMutationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	eventGen event.Interface,
	nsLister corev1listers.NamespaceLister,
	metrics metrics.MetricsConfigManager,
	admissionReports bool,
	reportsConfig reportutils.ReportingConfiguration,
) MutationHandler {
	return &mutationHandler{
		log:              log,
		kyvernoClient:    kyvernoClient,
		engine:           engine,
		eventGen:         eventGen,
		nsLister:         nsLister,
		metrics:          metrics,
		admissionReports: admissionReports,
		reportsConfig:    reportsConfig,
	}
}

type mutationHandler struct {
	log              logr.Logger
	kyvernoClient    versioned.Interface
	engine           engineapi.Engine
	eventGen         event.Interface
	nsLister         corev1listers.NamespaceLister
	metrics          metrics.MetricsConfigManager
	admissionReports bool
	reportsConfig    reportutils.ReportingConfiguration
}

func (h *mutationHandler) HandleMutation(
	ctx context.Context,
	request handlers.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
	cfg config.Configuration,
) ([]byte, []string, error) {
	mutatePatches, mutateEngineResponses, err := h.applyMutations(ctx, request, policies, policyContext, cfg)
	if err != nil {
		return nil, nil, err
	}
	if toggle.FromContext(ctx).DumpMutatePatches() {
		h.log.V(2).Info("", "generated patches", string(mutatePatches))
	}
	return mutatePatches, webhookutils.GetWarningMessages(mutateEngineResponses), nil
}

// applyMutations handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (v *mutationHandler) applyMutations(
	ctx context.Context,
	request handlers.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	cfg config.Configuration,
) ([]byte, []engineapi.EngineResponse, error) {
	if len(policies) == 0 {
		return nil, nil, nil
	}

	var patches []jsonpatch.JsonPatchOperation
	var engineResponses []engineapi.EngineResponse
	failurePolicy := kyvernov1.Ignore

	for _, policy := range policies {
		spec := policy.GetSpec()
		if !spec.HasMutateStandard() {
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

				engineResponse, policyPatches, err := v.applyMutation(ctx, request.AdmissionRequest, currentContext, failurePolicy, policy)
				if err != nil {
					return fmt.Errorf("mutation policy %s error: %v", policy.GetName(), err)
				}

				if len(policyPatches) > 0 {
					patches = append(patches, policyPatches...)
					rules := engineResponse.GetSuccessRules()
					if len(rules) != 0 {
						v.log.V(2).Info("mutation rules from policy applied successfully", "policy", policy.GetName(), "rules", rules)
					}
				}

				if engineResponse != nil {
					policyContext = currentContext.WithNewResource(engineResponse.PatchedResource)
					emitWarning := policy.GetSpec().EmitWarning
					if emitWarning != nil && *emitWarning {
						resp := engineResponse.WithWarning()
						engineResponse = &resp
					}
					engineResponses = append(engineResponses, *engineResponse)
				}

				return nil
			},
		)
		if err != nil {
			return nil, nil, err
		}
	}

	events := webhookutils.GenerateEvents(engineResponses, false, cfg)
	v.eventGen.Add(events...)

	go func() {
		if v.needsReports(request, v.admissionReports) {
			if err := v.createReports(context.TODO(), policyContext.NewResource(), request, engineResponses...); err != nil {
				v.log.Error(err, "failed to create report")
			}
		}
	}()

	logMutationResponse(patches, engineResponses, v.log)

	// patches holds all the successful patches, if no patch is created, it returns nil
	return jsonutils.JoinPatches(patch.ConvertPatches(patches...)...), engineResponses, nil
}

func (h *mutationHandler) applyMutation(ctx context.Context, request admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, failurePolicy kyvernov1.FailurePolicyType, policy kyvernov1.PolicyInterface) (*engineapi.EngineResponse, []jsonpatch.JsonPatchOperation, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels, err := engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, []kyvernov1.PolicyInterface{policy}, h.log)
		if err != nil {
			return nil, nil, err
		}

		policyContext = policyContext.WithNamespaceLabels(namespaceLabels)
	}

	engineResponse := h.engine.Mutate(ctx, policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() {
		if webhookutils.BlockRequest([]engineapi.EngineResponse{engineResponse}, failurePolicy, h.log) {
			h.log.V(2).Info("failed to apply policy, blocking request", "policy", policyContext.Policy().GetName(), "rules", engineResponse.GetFailedRulesWithErrors())
			return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy().GetName(), engineResponse.GetFailedRulesWithErrors())
		} else {
			h.log.V(4).Info("ignoring unsuccessful engine responses", "policy", policyContext.Policy().GetName(), "rules", engineResponse.GetFailedRulesWithErrors())
			return &engineResponse, nil, nil
		}
	}

	return &engineResponse, policyPatches, nil
}

func (h *mutationHandler) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	request handlers.AdmissionRequest,
	engineResponses ...engineapi.EngineResponse,
) error {
	report := reportutils.BuildMutationReport(resource, request.AdmissionRequest, engineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, h.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func logMutationResponse(patches []jsonpatch.JsonPatchOperation, engineResponses []engineapi.EngineResponse, logger logr.Logger) {
	if len(patches) != 0 {
		logger.V(4).Info("created patches", "count", len(patches))
	}
	// if any of the policies fails, print out the error
	if !engineutils.IsResponseSuccessful(engineResponses) {
		logger.Error(errors.New(webhookutils.GetErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
	}
}
