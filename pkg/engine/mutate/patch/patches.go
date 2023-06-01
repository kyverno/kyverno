package patch

import (
	"github.com/go-logr/logr"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

type (
	resource = []byte
	patches  = []jsonpatch.JsonPatchOperation
)

// Patcher patches the resource
type Patcher interface {
	Patch(logr.Logger, resource) (resource, patches, error)
}

// patchStrategicMergeHandler
type patchStrategicMergeHandler struct {
	patch apiextensions.JSON
}

func NewPatchStrategicMerge(patch apiextensions.JSON) Patcher {
	return patchStrategicMergeHandler{
		patch: patch,
	}
}

func (h patchStrategicMergeHandler) Patch(logger logr.Logger, resource resource) (resource, patches, error) {
	return ProcessStrategicMergePatch(logger, h.patch, resource)
}

// patchesJSON6902Handler
type patchesJSON6902Handler struct {
	patches string
}

func NewPatchesJSON6902(patches string) Patcher {
	return patchesJSON6902Handler{
		patches: patches,
	}
}

func (h patchesJSON6902Handler) Patch(logger logr.Logger, resource resource) (resource, patches, error) {
	patchesJSON6902, err := ConvertPatchesToJSON(h.patches)
	if err != nil {
		logger.Error(err, "error in type conversion")
		return nil, nil, err
	}
	return ProcessPatchJSON6902(logger, patchesJSON6902, resource)
}
