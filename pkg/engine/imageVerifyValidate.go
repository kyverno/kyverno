package engine

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
)

func processImageValidationRule(log logr.Logger, ctx *PolicyContext, rule *kyverno.Rule) *response.RuleResponse {
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
			for _, imageInfo := range infoMap {
				image := imageInfo.String()
				if !imageMatches(image, imageVerify.ImageReferences) {
					log.V(4).Info("image does not match pattern", "image", image, "patterns", imageVerify.ImageReferences)
					return nil
				}

				if err := validateImage(ctx, imageVerify, imageInfo); err != nil {
					return ruleResponse(*rule, response.ImageVerify, err.Error(), response.RuleStatusFail, nil)
				}
			}
		}
	}

	return ruleResponse(*rule, response.Validation, "image verified", response.RuleStatusPass, nil)
}

func validateImage(ctx *PolicyContext, imageVerify *kyverno.ImageVerification, imageInfo kubeutils.ImageInfo) error {
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.Log.Info("missing digest", "request.object", ctx.NewResource.UnstructuredContent())
		return fmt.Errorf("missing digest for %s", image)
	}

	if imageVerify.Required && !reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		verified, err := isImageVerified(ctx, imageInfo)
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

func isImageVerified(ctx *PolicyContext, imageInfo kubeutils.ImageInfo) (bool, error) {
	if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		return false, errors.Errorf("resource does not exist")
	}

	annotations := ctx.NewResource.GetAnnotations()
	if len(annotations) == 0 {
		return false, nil
	}

	key := makeAnnotationKey(imageInfo.Name)
	data, ok := annotations[key]
	if !ok {
		return false, errors.Errorf("image is not verified")
	}

	var ivm ImageVerificationMetadata
	if err := json.Unmarshal([]byte(data), &ivm); err != nil {
		return false, errors.Wrapf(err, "failed to extract image metadata")
	}

	if !ivm.Verified {
		return false, nil
	}

	if imageInfo.Digest != ivm.Digest {
		return false, errors.Errorf("failed to verify image; digest mismatch")
	}

	return true, nil
}
