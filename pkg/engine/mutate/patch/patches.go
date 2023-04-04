package patch

import (
	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Patcher patches the resource
type Patcher interface {
	Patch() (resp engineapi.RuleResponse, newPatchedResource unstructured.Unstructured)
}

// patchStrategicMergeHandler
type patchStrategicMergeHandler struct {
	ruleName        string
	patch           apiextensions.JSON
	patchedResource unstructured.Unstructured
	logger          logr.Logger
}

func NewPatchStrategicMerge(ruleName string, patch apiextensions.JSON, patchedResource unstructured.Unstructured, logger logr.Logger) Patcher {
	return patchStrategicMergeHandler{
		ruleName:        ruleName,
		patch:           patch,
		patchedResource: patchedResource,
		logger:          logger,
	}
}

func (h patchStrategicMergeHandler) Patch() (engineapi.RuleResponse, unstructured.Unstructured) {
	return ProcessStrategicMergePatch(h.ruleName, h.patch, h.patchedResource, h.logger)
}

// patchesJSON6902Handler
type patchesJSON6902Handler struct {
	ruleName        string
	patches         string
	patchedResource unstructured.Unstructured
	logger          logr.Logger
}

func NewPatchesJSON6902(ruleName string, patches string, patchedResource unstructured.Unstructured, logger logr.Logger) Patcher {
	return patchesJSON6902Handler{
		ruleName:        ruleName,
		patches:         patches,
		patchedResource: patchedResource,
		logger:          logger,
	}
}

func (h patchesJSON6902Handler) Patch() (engineapi.RuleResponse, unstructured.Unstructured) {
	patchesJSON6902, err := ConvertPatchesToJSON(h.patches)
	if err != nil {
		resp := engineapi.RuleFail(
			h.ruleName,
			engineapi.Mutation,
			err.Error(),
		)
		h.logger.Error(err, "error in type conversion")
		return *resp, unstructured.Unstructured{}
	}

	return ProcessPatchJSON6902(h.ruleName, patchesJSON6902, h.patchedResource, h.logger)
}
