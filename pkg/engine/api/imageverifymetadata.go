package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"gomodules.xyz/jsonpatch/v2"
)

type ImageVerificationMetadata struct {
	Data map[string]bool `json:"data"`
}

func (ivm *ImageVerificationMetadata) Add(image string, verified bool) {
	if ivm.Data == nil {
		ivm.Data = make(map[string]bool)
	}
	ivm.Data[image] = verified
}

func (ivm *ImageVerificationMetadata) IsVerified(image string) bool {
	if ivm.Data == nil {
		return false
	}
	verified, ok := ivm.Data[image]
	if !ok {
		return false
	}
	return verified
}

func ParseImageMetadata(jsonData string) (*ImageVerificationMetadata, error) {
	var data map[string]bool
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}
	return &ImageVerificationMetadata{
		Data: data,
	}, nil
}

func (ivm *ImageVerificationMetadata) Patches(hasAnnotations bool, log logr.Logger) ([]jsonpatch.JsonPatchOperation, error) {
	if data, err := json.Marshal(ivm.Data); err != nil {
		return nil, fmt.Errorf("failed to marshal metadata value: %v: %w", data, err)
	} else {
		var patches []jsonpatch.JsonPatchOperation
		if !hasAnnotations {
			patch := jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/annotations",
				Value:     map[string]string{},
			}
			log.V(4).Info("adding annotation patch", "patch", patch)
			patches = append(patches, patch)
		}
		patch := jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      makeAnnotationKeyForJSONPatch(),
			Value:     string(data),
		}
		log.V(4).Info("adding image verification patch", "patch", patch)
		patches = append(patches, patch)
		return patches, nil
	}
}

func (ivm *ImageVerificationMetadata) Merge(other ImageVerificationMetadata) {
	for k, v := range other.Data {
		ivm.Add(k, v)
	}
}

func (ivm *ImageVerificationMetadata) IsEmpty() bool {
	return len(ivm.Data) == 0
}

func makeAnnotationKeyForJSONPatch() string {
	return "/metadata/annotations/" + strings.ReplaceAll(kyverno.AnnotationImageVerify, "/", "~1")
}
