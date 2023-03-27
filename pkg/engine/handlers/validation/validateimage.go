package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type validateImageHandler struct {
	configuration config.Configuration
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader
}

func NewValidateImageHandler(
	contextLoader func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.EngineContextLoader,
) handlers.Handler {
	return validateImageHandler{
		contextLoader: contextLoader,
	}
}

func (h validateImageHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	if engineutils.IsDeleteRequest(policyContext) {
		return resource, nil
	}
	policy := policyContext.Policy()
	contextLoader := h.contextLoader(policy, rule)
	matchingImages, _, err := engineutils.ExtractMatchingImages(
		policyContext.NewResource(),
		policyContext.JSONContext(),
		rule,
		h.configuration,
	)
	if err != nil {
		return resource, handlers.RuleResponses(internal.RuleError(&rule, engineapi.Validation, "", err))
	}
	if len(matchingImages) == 0 {
		return resource, handlers.RuleResponses(internal.RuleSkip(&rule, engineapi.Validation, "image verified"))
	}
	if err := contextLoader(ctx, rule.Context, policyContext.JSONContext()); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			logger.Error(err, "failed to load context")
		}

		return resource, handlers.RuleResponses(internal.RuleError(&rule, engineapi.Validation, "failed to load context", err))
	}
	preconditionsPassed, err := internal.CheckPreconditions(logger, policyContext, rule.RawAnyAllConditions)
	if err != nil {
		return resource, handlers.RuleResponses(internal.RuleError(&rule, engineapi.Validation, "failed to evaluate preconditions", err))
	}
	if !preconditionsPassed {
		if policyContext.Policy().GetSpec().ValidationFailureAction.Audit() {
			return resource, nil
		}

		return resource, handlers.RuleResponses(internal.RuleSkip(&rule, engineapi.Validation, "preconditions not met"))
	}
	for _, v := range rule.VerifyImages {
		imageVerify := v.Convert()
		for _, infoMap := range policyContext.JSONContext().ImageInfo() {
			for name, imageInfo := range infoMap {
				image := imageInfo.String()

				if !engineutils.ImageMatches(image, imageVerify.ImageReferences) {
					logger.V(4).Info("image does not match", "imageReferences", imageVerify.ImageReferences)
					return resource, nil
				}

				logger.V(4).Info("validating image", "image", image)
				if err := validateImage(policyContext, imageVerify, name, imageInfo, logger); err != nil {
					return resource, handlers.RuleResponses(internal.RuleResponse(rule, engineapi.ImageVerify, err.Error(), engineapi.RuleStatusFail))
				}
			}
		}
	}
	logger.V(4).Info("validated image", "rule", rule.Name)
	return resource, handlers.RuleResponses(internal.RulePass(&rule, engineapi.Validation, "image verified"))
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
