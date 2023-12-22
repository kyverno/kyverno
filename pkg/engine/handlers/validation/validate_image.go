package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type validateImageHandler struct{}

func NewValidateImageHandler(
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	configuration config.Configuration,
) (handlers.Handler, error) {
	if engineutils.IsDeleteRequest(policyContext) {
		return nil, nil
	}
	ruleImages, _, err := engineutils.ExtractMatchingImages(resource, policyContext.JSONContext(), rule, configuration)
	if err != nil {
		return nil, err
	}
	if len(ruleImages) == 0 {
		return nil, nil
	}
	return validateImageHandler{}, nil
}

func (h validateImageHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	_ engineapi.EngineContextLoader,
	exceptions []kyvernov2beta1.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there is a policy exception matches the incoming resource
	exception := engineutils.MatchesException(exceptions, policyContext, logger)
	if exception != nil {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
		} else {
			logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule skipped due to policy exception "+key).WithException(exception),
			)
		}
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
					return resource, handlers.WithFail(rule, engineapi.ImageVerify, err.Error())
				}
			}
		}
	}
	logger.V(4).Info("validated image", "rule", rule.Name)
	return resource, handlers.WithPass(rule, engineapi.Validation, "image verified")
}

func validateImage(ctx engineapi.PolicyContext, imageVerify *kyvernov1.ImageVerification, name string, imageInfo apiutils.ImageInfo, log logr.Logger) error {
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.V(2).Info("missing digest", "image", imageInfo.String())
		return fmt.Errorf("missing digest for %s", image)
	}
	newResource := ctx.NewResource()
	if imageVerify.Required && newResource.Object != nil {
		verified, err := engineutils.IsImageVerified(newResource, image, log)
		if err != nil {
			return err
		}
		if !verified {
			return fmt.Errorf("unverified image %s", image)
		}
	}
	return nil
}
