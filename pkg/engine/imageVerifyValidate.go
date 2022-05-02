package engine

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
)

func processImageValidationRule(log logr.Logger, ctx *PolicyContext, rule *kyverno.Rule) *response.RuleResponse {
	if isDeleteRequest(ctx) {
		return nil
	}

	if err := LoadContext(log, rule.Context, ctx, rule.Name); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			log.Error(err, "failed to load context")
		}

		return ruleError(rule, response.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(log, ctx, rule.RawAnyAllConditions)
	if err != nil {
		return ruleError(rule, response.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		if ctx.Policy.GetSpec().ValidationFailureAction == kyverno.Audit {
			return nil
		}

		return ruleResponse(*rule, response.Validation, "preconditions not met", response.RuleStatusSkip, nil)
	}

	for _, v := range rule.VerifyImages {
		imageVerify := v.Convert()
		for _, infoMap := range ctx.JSONContext.ImageInfo() {
			for containerName, imageInfo := range infoMap {
				image := imageInfo.String()
				if !imageMatches(image, imageVerify.ImageReferences) {
					log.V(4).Info("image does not match pattern", "image", image, "patterns", imageVerify.ImageReferences)
					return nil
				}

				if err := validateImage(ctx, imageVerify, containerName, imageInfo, log); err != nil {
					return ruleResponse(*rule, response.ImageVerify, err.Error(), response.RuleStatusFail, nil)
				}
			}
		}
	}

	return ruleResponse(*rule, response.Validation, "image verified", response.RuleStatusPass, nil)
}

func validateImage(ctx *PolicyContext, imageVerify *kyverno.ImageVerification, containerName string, imageInfo kubeutils.ImageInfo, log logr.Logger) error {
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.Info("missing digest", "image", imageInfo.String())
		return fmt.Errorf("missing digest for %s", image)
	}

	if imageVerify.Required && !reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		verified, err := isImageVerified(ctx, containerName, imageInfo, log)
		if err != nil {
			return err
		}

		if !verified {
			return fmt.Errorf("unverified image %s", image)
		}
	}

	return nil
}

type ImageVerificationMetadata struct {
	Verified bool   `json:"verified,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

func isImageVerified(ctx *PolicyContext, containerName string, imageInfo kubeutils.ImageInfo, log logr.Logger) (bool, error) {
	if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		return false, errors.Errorf("resource does not exist")
	}

	annotations := ctx.NewResource.GetAnnotations()
	if len(annotations) == 0 {
		return false, nil
	}

	key := makeAnnotationKey(containerName)
	data, ok := annotations[key]
	if !ok {
		log.V(2).Info("missing image metadata in annotation", "key", key)
		return false, errors.Errorf("image is not verified")
	}

	var ivm ImageVerificationMetadata
	if err := json.Unmarshal([]byte(data), &ivm); err != nil {
		log.Error(err, "failed to parse image verification metadata", "data", data)
		return false, errors.Wrapf(err, "failed to parse image metadata")
	}

	if !ivm.Verified {
		return false, nil
	}

	if imageInfo.Digest != ivm.Digest {
		log.V(2).Info("digest mismatch", "image", imageInfo.String(), "expected", imageInfo.Digest, "received", ivm.Digest)
		return false, errors.Errorf("failed to verify image; digest mismatch")
	}

	return true, nil
}
