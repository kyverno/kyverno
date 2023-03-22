package engine

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (e *engine) processImageValidationRule(
	ctx context.Context,
	log logr.Logger,
	enginectx engineapi.PolicyContext,
	rule *kyvernov1.Rule,
) *engineapi.RuleResponse {
	if isDeleteRequest(enginectx) {
		return nil
	}

	log = log.WithValues("rule", rule.Name)
	matchingImages, _, err := e.extractMatchingImages(enginectx, rule)
	if err != nil {
		return internal.RuleError(rule, engineapi.Validation, "", err)
	}
	if len(matchingImages) == 0 {
		return internal.RuleSkip(rule, engineapi.Validation, "image verified")
	}
	if err := internal.LoadContext(ctx, e, enginectx, *rule); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			log.Error(err, "failed to load context")
		}

		return internal.RuleError(rule, engineapi.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := internal.CheckPreconditions(log, enginectx, rule.RawAnyAllConditions)
	if err != nil {
		return internal.RuleError(rule, engineapi.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		if enginectx.Policy().GetSpec().ValidationFailureAction.Audit() {
			return nil
		}

		return internal.RuleSkip(rule, engineapi.Validation, "preconditions not met")
	}

	for _, v := range rule.VerifyImages {
		imageVerify := v.Convert()
		for _, infoMap := range enginectx.JSONContext().ImageInfo() {
			for name, imageInfo := range infoMap {
				image := imageInfo.String()
				log = log.WithValues("rule", rule.Name)

				if !imageMatches(image, imageVerify.ImageReferences) {
					log.V(4).Info("image does not match", "imageReferences", imageVerify.ImageReferences)
					return nil
				}

				log.V(4).Info("validating image", "image", image)
				if err := validateImage(enginectx, imageVerify, name, imageInfo, log); err != nil {
					return internal.RuleResponse(*rule, engineapi.ImageVerify, err.Error(), engineapi.RuleStatusFail)
				}
			}
		}
	}

	log.V(4).Info("validated image", "rule", rule.Name)
	return internal.RulePass(rule, engineapi.Validation, "image verified")
}

func validateImage(ctx engineapi.PolicyContext, imageVerify *kyvernov1.ImageVerification, name string, imageInfo apiutils.ImageInfo, log logr.Logger) error {
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.V(2).Info("missing digest", "image", imageInfo.String())
		return fmt.Errorf("missing digest for %s", image)
	}
	newResource := ctx.NewResource()
	if imageVerify.Required && newResource.Object != nil {
		verified, err := isImageVerified(newResource, image, log)
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
	if resource.Object == nil {
		return false, fmt.Errorf("nil resource")
	}
	if annotations := resource.GetAnnotations(); len(annotations) == 0 {
		return false, nil
	} else if data, ok := annotations[engineapi.ImageVerifyAnnotationKey]; !ok {
		log.V(2).Info("missing image metadata in annotation", "key", engineapi.ImageVerifyAnnotationKey)
		return false, fmt.Errorf("image is not verified")
	} else if ivm, err := engineapi.ParseImageMetadata(data); err != nil {
		log.Error(err, "failed to parse image verification metadata", "data", data)
		return false, fmt.Errorf("failed to parse image metadata: %w", err)
	} else {
		return ivm.IsVerified(image), nil
	}
}
