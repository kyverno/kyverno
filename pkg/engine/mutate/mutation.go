package mutate

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Handler knows how to mutate resources with given pattern
type Handler interface {
	Handle() (resp response.RuleResponse, newPatchedResource unstructured.Unstructured)
}

// CreateMutateHandler initilizes a new instance of mutation handler
func CreateMutateHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) Handler {

	switch {
	case isPatchStrategicMerge(mutate):
		return newPatchStrategicMergeHandler(ruleName, mutate, patchedResource, context, logger)
	case isPatchesJSON6902(mutate):
		return newPatchesJSON6902Handler(ruleName, mutate, patchedResource, logger)
	case isOverlay(mutate):
		// return newOverlayHandler(ruleName, mutate, patchedResource, context, logger)
		mutate.PatchStrategicMerge = mutate.Overlay
		var a []byte
		mutate.Overlay.Raw = a
		return newPatchStrategicMergeHandler(ruleName, mutate, patchedResource, context, logger)
	case isPatches(mutate):
		return newPatchesHandler(ruleName, mutate, patchedResource, context, logger)
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

func newPatchStrategicMergeHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) Handler {
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

func newOverlayHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) Handler {
	return overlayHandler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		evalCtx:         context,
		logger:          logger,
	}
}

// patchesJSON6902Handler
type patchesJSON6902Handler struct {
	ruleName        string
	mutation        *kyverno.Mutation
	patchedResource unstructured.Unstructured
	evalCtx         context.EvalInterface
	logger          logr.Logger
}

func newPatchesJSON6902Handler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, logger logr.Logger) Handler {
	return patchesJSON6902Handler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		logger:          logger,
	}
}

func (h patchesJSON6902Handler) Handle() (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	resp.Name = h.ruleName
	resp.Type = utils.Mutation.String()

	patchesJSON6902, err := convertPatchesToJSON(h.mutation.PatchesJSON6902)
	if err != nil {
		resp.Success = false
		h.logger.Error(err, "error in type conversion")
		resp.Message = err.Error()
		return resp, h.patchedResource
	}

	return ProcessPatchJSON6902(h.ruleName, patchesJSON6902, h.patchedResource, h.logger)
}

func (h overlayHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	return ProcessOverlay(h.logger, h.ruleName, h.mutation.Overlay, h.patchedResource)
}

// patchesHandler
type patchesHandler struct {
	ruleName        string
	mutation        *kyverno.Mutation
	patchedResource unstructured.Unstructured
	evalCtx         context.EvalInterface
	logger          logr.Logger
}

func newPatchesHandler(ruleName string, mutate *kyverno.Mutation, patchedResource unstructured.Unstructured, context context.EvalInterface, logger logr.Logger) Handler {
	return patchesHandler{
		ruleName:        ruleName,
		mutation:        mutate,
		patchedResource: patchedResource,
		evalCtx:         context,
		logger:          logger,
	}
}

func (h patchesHandler) Handle() (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	resp.Name = h.ruleName
	resp.Type = utils.Mutation.String()

	return ProcessPatches(h.logger, h.ruleName, *h.mutation, h.patchedResource)
}

// emptyHandler
type emptyHandler struct {
	patchedResource unstructured.Unstructured
}

func newEmptyHandler(patchedResource unstructured.Unstructured) Handler {
	return emptyHandler{
		patchedResource: patchedResource,
	}
}

func (h emptyHandler) Handle() (response.RuleResponse, unstructured.Unstructured) {
	return response.RuleResponse{}, h.patchedResource
}

func isPatchStrategicMerge(mutate *kyverno.Mutation) bool {
	if mutate.PatchStrategicMerge.Raw != nil {
		return true
	}
	return false
}

func isPatchesJSON6902(mutate *kyverno.Mutation) bool {
	if len(mutate.PatchesJSON6902) > 0 {
		return true
	}
	return false
}

func isOverlay(mutate *kyverno.Mutation) bool {
	if mutate.Overlay.Raw != nil {
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
