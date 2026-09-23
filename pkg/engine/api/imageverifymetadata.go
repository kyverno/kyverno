package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"gomodules.xyz/jsonpatch/v2"
)

type ImageVerificationMetadataStatus string

const (
	ImageVerificationPass ImageVerificationMetadataStatus = "pass"
	ImageVerificationFail ImageVerificationMetadataStatus = "fail"
	ImageVerificationSkip ImageVerificationMetadataStatus = "skip"
)

type ImageVerificationMetadata struct {
	Data     map[string]ImageVerificationMetadataStatus
	Policies map[string]map[string]map[string]ImageVerificationMetadataStatus
}

type scopedImageVerificationMetadata struct {
	Version  int                                                              `json:"version"`
	Policies map[string]map[string]map[string]ImageVerificationMetadataStatus `json:"policies"`
}

func imageVerificationPolicyKey(namespace, name string) string {
	return namespace + "/" + name
}

func (ivm *ImageVerificationMetadata) Add(image string, verified ImageVerificationMetadataStatus) {
	if ivm.Data == nil {
		ivm.Data = make(map[string]ImageVerificationMetadataStatus)
	}
	ivm.Data[image] = verified
}

func (ivm *ImageVerificationMetadata) AddScoped(namespace, policy, rule, image string, verified ImageVerificationMetadataStatus) {
	if ivm.Policies == nil {
		ivm.Policies = make(map[string]map[string]map[string]ImageVerificationMetadataStatus)
	}
	policyKey := imageVerificationPolicyKey(namespace, policy)
	if ivm.Policies[policyKey] == nil {
		ivm.Policies[policyKey] = make(map[string]map[string]ImageVerificationMetadataStatus)
	}
	if ivm.Policies[policyKey][rule] == nil {
		ivm.Policies[policyKey][rule] = make(map[string]ImageVerificationMetadataStatus)
	}
	ivm.Policies[policyKey][rule][image] = verified
}

func (ivm *ImageVerificationMetadata) IsVerified(image string) bool {
	if ivm.Data == nil {
		return false
	}
	verified, ok := ivm.Data[image]
	if !ok {
		return false
	}
	return verified == ImageVerificationPass || verified == ImageVerificationSkip
}

func (ivm *ImageVerificationMetadata) ImageVerificationStatus(image string) ImageVerificationMetadataStatus {
	if verified, ok := ivm.Data[image]; ok {
		return verified
	}
	return ImageVerificationFail
}

func (ivm *ImageVerificationMetadata) ScopedImageVerificationStatus(namespace, policy, rule, image string) ImageVerificationMetadataStatus {
	if rules := ivm.Policies[imageVerificationPolicyKey(namespace, policy)]; rules != nil {
		if images := rules[rule]; images != nil {
			if verified, ok := images[image]; ok {
				return verified
			}
		}
	}
	return ImageVerificationFail
}

func ParseImageMetadata(jsonData string) (*ImageVerificationMetadata, error) {
	var scoped scopedImageVerificationMetadata
	if err := json.Unmarshal([]byte(jsonData), &scoped); err == nil && scoped.Version == 1 && scoped.Policies != nil {
		return &ImageVerificationMetadata{Policies: scoped.Policies}, nil
	}
	var data map[string]ImageVerificationMetadataStatus
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
		if ivm.Policies != nil {
			scopedData, err := json.Marshal(scopedImageVerificationMetadata{Version: 1, Policies: ivm.Policies})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal scoped metadata value: %v: %w", scopedData, err)
			}
			scopedPatch := jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      makeScopedAnnotationKeyForJSONPatch(),
				Value:     string(scopedData),
			}
			log.V(4).Info("adding scoped image verification patch", "patch", scopedPatch)
			patches = append(patches, scopedPatch)
		}
		return patches, nil
	}
}

func (ivm *ImageVerificationMetadata) Merge(other ImageVerificationMetadata) {
	for k, v := range other.Data {
		ivm.Add(k, v)
	}
	for policy, rules := range other.Policies {
		for rule, images := range rules {
			for image, status := range images {
				if ivm.Policies == nil {
					ivm.Policies = make(map[string]map[string]map[string]ImageVerificationMetadataStatus)
				}
				if ivm.Policies[policy] == nil {
					ivm.Policies[policy] = make(map[string]map[string]ImageVerificationMetadataStatus)
				}
				if ivm.Policies[policy][rule] == nil {
					ivm.Policies[policy][rule] = make(map[string]ImageVerificationMetadataStatus)
				}
				ivm.Policies[policy][rule][image] = status
			}
		}
	}
}

func (ivm *ImageVerificationMetadata) IsEmpty() bool {
	return len(ivm.Data) == 0 && len(ivm.Policies) == 0
}

func makeAnnotationKeyForJSONPatch() string {
	return "/metadata/annotations/" + strings.ReplaceAll(kyverno.AnnotationImageVerify, "/", "~1")
}

func makeScopedAnnotationKeyForJSONPatch() string {
	return "/metadata/annotations/" + strings.ReplaceAll(kyverno.AnnotationImageVerifyScoped, "/", "~1")
}
