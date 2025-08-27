package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	policyName := policyContext.Policy().GetName()
	if policyContext.Policy().GetNamespace() != "" {
		policyName = policyContext.Policy().GetNamespace() + "/" + policyName
	}

	// check if there are general policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		// Check if any of the matched exceptions have fine-grained criteria
		hasFinegrainedExceptions := false
		for _, exception := range matchedExceptions {
			for _, exc := range exception.Spec.Exceptions {
				if exc.Contains(policyName, rule.Name) && exc.IsFinegrained() {
					hasFinegrainedExceptions = true
					break
				}
			}
			if hasFinegrainedExceptions {
				break
			}
		}

		// If no fine-grained exceptions, use the old behavior (skip entire rule)
		if !hasFinegrainedExceptions {
			exceptions := make([]engineapi.GenericException, 0, len(matchedExceptions))
			var keys []string
			for i, exception := range matchedExceptions {
				key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
				if err != nil {
					logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
					return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
				}
				keys = append(keys, key)
				exceptions = append(exceptions, engineapi.NewPolicyException(&exception))
			}

			logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(exceptions),
			)
		}
	}

	skippedImages := make([]string, 0)
	passedImages := make([]string, 0)
	exemptedImages := make([]string, 0)
	exemptedReportMode := kyvernov2.ExceptionReportSkip

	for _, v := range rule.VerifyImages {
		imageVerify := v.Convert()
		for _, infoMap := range policyContext.JSONContext().ImageInfo() {
			for _, imageInfo := range infoMap {
				image := imageInfo.String()

				if !engineutils.ImageMatches(image, imageVerify.ImageReferences) {
					logger.V(4).Info("image does not match", "imageReferences", imageVerify.ImageReferences)
					return resource, nil
				}

				// Check for fine-grained image-based exceptions
				finegrainedExceptions, reportMode := engineutils.MatchesFinegrainedException(
					exceptions, policyContext, policyName, rule.Name, logger,
				)

				// Check if this specific image is exempted
				imageExempted := false
				if len(finegrainedExceptions) > 0 {
					for _, exception := range finegrainedExceptions {
						for _, exc := range exception.Spec.Exceptions {
							if exc.Contains(policyName, rule.Name) && exc.HasImageExceptions() {
								for _, imgExc := range exc.Images {
									for _, imgRef := range imgExc.ImageReferences {
										if engineutils.ImageMatches(image, []string{imgRef}) {
											imageExempted = true
											exemptedImages = append(exemptedImages, image)
											exemptedReportMode = reportMode
											logger.V(4).Info("image exempted by fine-grained exception", "image", image, "pattern", imgRef, "reportMode", reportMode)
											break
										}
									}
									if imageExempted {
										break
									}
								}
							}
							if imageExempted {
								break
							}
						}
						if imageExempted {
							break
						}
					}
				}

				if imageExempted {
					continue // Skip validation for this image
				}

				logger.V(4).Info("validating image", "image", image)
				if v, err := validateImage(policyContext, imageVerify, imageInfo, logger); err != nil {
					return resource, handlers.WithFail(rule, engineapi.ImageVerify, err.Error())
				} else if v == engineapi.ImageVerificationSkip {
					skippedImages = append(skippedImages, image)
				} else if v == engineapi.ImageVerificationPass {
					passedImages = append(passedImages, image)
				}
			}
		}
	}

	logger.V(4).Info("validated image", "rule", rule.Name)

	// Handle exempted images based on their reporting mode
	if len(exemptedImages) > 0 {
		var exemptedExceptions []engineapi.GenericException
		for _, exception := range matchedExceptions {
			exemptedExceptions = append(exemptedExceptions, engineapi.NewPolicyException(&exception))
		}

		switch exemptedReportMode {
		case kyvernov2.ExceptionReportWarn:
			msg := fmt.Sprintf("images exempted with warning: %s", strings.Join(exemptedImages, ", "))
			return resource, handlers.WithResponses(
				engineapi.RuleWarn(rule.Name, engineapi.ImageVerify, msg, rule.ReportProperties).WithExceptions(exemptedExceptions),
			)
		case kyvernov2.ExceptionReportPass:
			msg := fmt.Sprintf("images exempted and reported as pass: %s", strings.Join(exemptedImages, ", "))
			return resource, handlers.WithResponses(
				engineapi.RulePass(rule.Name, engineapi.ImageVerify, msg, rule.ReportProperties).WithExceptions(exemptedExceptions),
			)
		default: // kyvernov2.ExceptionReportSkip
			msg := fmt.Sprintf("images exempted: %s", strings.Join(exemptedImages, ", "))
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.ImageVerify, msg, rule.ReportProperties).WithExceptions(exemptedExceptions),
			)
		}
	}

	if len(passedImages) > 0 || len(passedImages)+len(skippedImages) == 0 {
		if len(skippedImages) > 0 {
			return resource, handlers.WithPass(rule, engineapi.ImageVerify, strings.Join(append([]string{"image verified, skipped images:"}, skippedImages...), " "))
		}
		return resource, handlers.WithPass(rule, engineapi.ImageVerify, "image verified")
	} else {
		return resource, handlers.WithSkip(rule, engineapi.ImageVerify, strings.Join(append([]string{"image skipped, skipped images:"}, skippedImages...), " "))
	}
}

func validateImage(ctx engineapi.PolicyContext, imageVerify *kyvernov1.ImageVerification, imageInfo apiutils.ImageInfo, log logr.Logger) (engineapi.ImageVerificationMetadataStatus, error) {
	var verified engineapi.ImageVerificationMetadataStatus
	var err error
	image := imageInfo.String()
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		log.V(2).Info("missing digest", "image", imageInfo.String())
		return engineapi.ImageVerificationFail, fmt.Errorf("missing digest for %s", image)
	}
	newResource := ctx.NewResource()
	if imageVerify.Required && newResource.Object != nil {
		verified, err = engineutils.IsImageVerified(newResource, image, log)
		if err != nil {
			return engineapi.ImageVerificationFail, err
		}
		if verified == engineapi.ImageVerificationFail {
			return engineapi.ImageVerificationFail, fmt.Errorf("unverified image %s", image)
		}
	}
	return verified, nil
}
