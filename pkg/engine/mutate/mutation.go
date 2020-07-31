package mutate

import (
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type MutateHandler interface {
	Handle() (resp response.RuleResponse, newPatchedResource unstructured.Unstructured)
}

func CreateMutateHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) MutateHandler {

	switch {
	case isPatchStrategicMerge(mutate):
		return newpatchStrategicMergeHandler(ruleName, mutate, patchedResource, context, logger)
	case isOverlay(mutate):
		return newOverlayHandler(ruleName, mutate, patchedResource, context, logger)
	case isPatches(mutate):
		return newpatchesHandler(ruleName, mutate, patchedResource, context, logger)
	default:
		return newEmptyHandler(patchedResource)
	}
}

// patchStrategicMergeHandler
type patchStrategicMergeHandler struct {
	ruleName        string
	mutation        *kyverno.Mutation
	patchedResource unstructured.Unstructured
	evalCtx         context.EvalInterface
	logger          logr.Logger
}

func newpatchStrategicMergeHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) MutateHandler {
	return patchStrategicMergeHandler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		evalCtx:         context,
		logger:          logger,
	}
}

func (h patchStrategicMergeHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	return ProcessStrategicMergePatch(h.ruleName, h.mutation.PatchStrategicMerge, h.patchedResource, h.logger)

}

// overlayHandler
type overlayHandler struct {
	ruleName        string
	mutation        *kyverno.Mutation
	patchedResource unstructured.Unstructured
	evalCtx         context.EvalInterface
	logger          logr.Logger
}

func newOverlayHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) MutateHandler {
	return overlayHandler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		evalCtx:         context,
		logger:          logger,
	}
}

func (h overlayHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	var ruleResponse response.RuleResponse
	overlay := h.mutation.Overlay

	// substitute the variables
	var err error
	if overlay, err = variables.SubstituteVars(h.logger, h.evalCtx, overlay); err != nil {
		// variable subsitution failed
		ruleResponse.Success = false
		ruleResponse.Message = err.Error()
		return ruleResponse, h.patchedResource
	}

	return ProcessOverlay(h.logger, h.ruleName, overlay, h.patchedResource)
}

// patchesHandler
type patchesHandler struct {
	ruleName        string
	mutation        *kyverno.Mutation
	patchedResource unstructured.Unstructured
	evalCtx         context.EvalInterface
	logger          logr.Logger
}

func newpatchesHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) MutateHandler {
	return patchesHandler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		evalCtx:         context,
		logger:          logger,
	}
}

func (h patchesHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	return ProcessPatches(h.logger, h.ruleName, *h.mutation, h.patchedResource)
}

// emptyHandler
type emptyHandler struct {
	patchedResource unstructured.Unstructured
}

func newEmptyHandler(patchedResource unstructured.Unstructured) MutateHandler {
	return emptyHandler{
		patchedResource: patchedResource,
	}
}

func (h emptyHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	return response.RuleResponse{}, h.patchedResource
}

func isPatchStrategicMerge(mutate *kyverno.Mutation) bool {
	if mutate.PatchStrategicMerge != nil {
		return true
	}
	return false
}

func isOverlay(mutate *kyverno.Mutation) bool {
	if mutate.Overlay != nil {
		return true
	}
	return false
}

func isPatches(mutate *kyverno.Mutation) bool {
	if len(mutate.Patches) != 0 {
		return true
	}
	return false
}
