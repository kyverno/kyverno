package engine

import (
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const imageVerifyAnnotationKey = "kyverno.io/verify-images"

type ImageVerificationMetadata struct {
	Data map[string]bool `json:"data"`
}

func (ivm *ImageVerificationMetadata) add(image string, verified bool) {
	if ivm.Data == nil {
		ivm.Data = make(map[string]bool)
	}

	ivm.Data[image] = verified
}

func (ivm *ImageVerificationMetadata) isVerified(image string) bool {
	if ivm.Data == nil {
		return false
	}

	verified, ok := ivm.Data[image]
	if !ok {
		return false
	}

	return verified
}

func parseImageMetadata(jsonData string) (*ImageVerificationMetadata, error) {
	var data map[string]bool
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}

	return &ImageVerificationMetadata{
		Data: data,
	}, nil
}

func (ivm *ImageVerificationMetadata) Patches(hasAnnotations bool, log logr.Logger) ([][]byte, error) {
	var patches [][]byte
	if !hasAnnotations {
		addAnnotationsPatch := make(map[string]interface{})
		addAnnotationsPatch["op"] = "add"
		addAnnotationsPatch["path"] = "/metadata/annotations"
		addAnnotationsPatch["value"] = map[string]string{}
		patchBytes, err := json.Marshal(addAnnotationsPatch)
		if err != nil {
			return nil, err
		}

		log.V(4).Info("adding annotation patch", "patch", string(patchBytes))
		patches = append(patches, patchBytes)
	}

	data, err := json.Marshal(ivm.Data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal metadata value: %v", data)
	}

	addKeyPatch := make(map[string]interface{})
	addKeyPatch["op"] = "add"
	addKeyPatch["path"] = makeAnnotationKeyForJSONPatch()
	addKeyPatch["value"] = string(data)

	patchBytes, err := json.Marshal(addKeyPatch)
	if err != nil {
		return nil, err
	}

	log.V(4).Info("adding image verification patch", "patch", string(patchBytes))
	patches = append(patches, patchBytes)
	return patches, nil
}

func (ivm *ImageVerificationMetadata) Merge(other *ImageVerificationMetadata) {
	for k, v := range other.Data {
		ivm.add(k, v)
	}
}

func (ivm *ImageVerificationMetadata) IsEmpty() bool {
	return len(ivm.Data) == 0
}

func makeAnnotationKeyForJSONPatch() string {
	return "/metadata/annotations/" + strings.ReplaceAll(imageVerifyAnnotationKey, "/", "~1")
}
