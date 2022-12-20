package engine

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/registryclient"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func processImageValidationRule(ctx context.Context, log logr.Logger, rclient registryclient.Client, enginectx *PolicyContext, rule *kyvernov1.Rule) *response.RuleResponse {
	if isDeleteRequest(enginectx) {
		return nil
	}

	log = log.WithValues("rule", rule.Name)
	matchingImages, _, err := extractMatchingImages(enginectx, rule)
	if err != nil {
		return ruleResponse(*rule, response.Validation, err.Error(), response.RuleStatusError)
	}
	if len(matchingImages) == 0 {
		return ruleResponse(*rule, response.Validation, "image verified", response.RuleStatusSkip)
	}
	if err := LoadContext(ctx, log, rclient, rule.Context, enginectx, rule.Name); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			log.Error(err, "failed to load context")
		}

		return ruleError(rule, response.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(log, enginectx, rule.RawAnyAllConditions)
	if err != nil {
		return ruleError(rule, response.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		if enginectx.policy.GetSpec().ValidationFailureAction.Audit() {
			return nil
		}

		return ruleResponse(*rule, response.Validation, "preconditions not met", response.RuleStatusSkip)
	}

	for _, v := range rule.VerifyImages {
		imageVerify := v.Convert()
		for _, infoMap := range enginectx.jsonContext.ImageInfo() {
			for name, imageInfo := range infoMap {
				image := imageInfo.String()
				log = log.WithValues("rule", rule.Name)

				if !imageMatches(image, imageVerify.ImageReferences) {
					log.V(4).Info("image does not match", "imageReferences", imageVerify.ImageReferences)
					return nil
				}

				log.V(4).Info("validating image", "image", image)
				if err := validateImage(enginectx, imageVerify, name, imageInfo, log); err != nil {
					return ruleResponse(*rule, response.ImageVerify, err.Error(), response.RuleStatusFail)
				}
			}
		}
	}

	log.V(4).Info("validated image", "rule", rule.Name)
	return ruleResponse(*rule, response.Validation, "image verified", response.RuleStatusPass)
}

func validateImage(ctx *PolicyContext, imageVerify *kyvernov1.ImageVerification, name string, imageInfo apiutils.ImageInfo, log logr.Logger) error {
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.V(2).Info("missing digest", "image", imageInfo.String())
		return fmt.Errorf("missing digest for %s", image)
	}

	if imageVerify.Required && !reflect.DeepEqual(ctx.newResource, unstructured.Unstructured{}) {
		verified, err := isImageVerified(ctx.newResource, image, log)
		if err != nil {
			return err
		}

		if !verified {
			return fmt.Errorf("unverified image %s", image)
		}
	}

	return nil
}

func isImageVerified(resource unstructured.Unstructured, image string, log logr.Logger) (bool, error) {
	if reflect.DeepEqual(resource, unstructured.Unstructured{}) {
		return false, errors.Errorf("nil resource")
	}

	annotations := resource.GetAnnotations()
	if len(annotations) == 0 {
		return false, nil
	}

	key := imageVerifyAnnotationKey
	data, ok := annotations[key]
	if !ok {
		log.V(2).Info("missing image metadata in annotation", "key", key)
		return false, errors.Errorf("image is not verified")
	}

	ivm, err := parseImageMetadata(data)
	if err != nil {
		log.Error(err, "failed to parse image verification metadata", "data", data)
		return false, errors.Wrapf(err, "failed to parse image metadata")
	}

	return ivm.isVerified(image), nil
}
