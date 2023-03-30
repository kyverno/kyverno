package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
)

const ImageVerifyAnnotationKey = "kyverno.io/verify-images"

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

func (ivm *ImageVerificationMetadata) Patches(hasAnnotations bool, log logr.Logger) ([][]byte, error) {
	if data, err := json.Marshal(ivm.Data); err != nil {
		return nil, fmt.Errorf("failed to marshal metadata value: %v: %w", data, err)
	} else {
		var patches [][]byte
		if !hasAnnotations {
			patch := jsonutils.NewPatchOperation("/metadata/annotations", "add", map[string]string{})
			patchBytes, err := patch.Marshal()
			if err != nil {
				return nil, err
			}
			log.V(4).Info("adding annotation patch", "patch", patch)
			patches = append(patches, patchBytes)
		}
		patch := jsonutils.NewPatchOperation(makeAnnotationKeyForJSONPatch(), "add", string(data))
		patchBytes, err := patch.Marshal()
		if err != nil {
			return nil, err
		}
		log.V(4).Info("adding image verification patch", "patch", patch)
		patches = append(patches, patchBytes)
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
	return "/metadata/annotations/" + strings.ReplaceAll(ImageVerifyAnnotationKey, "/", "~1")
}
