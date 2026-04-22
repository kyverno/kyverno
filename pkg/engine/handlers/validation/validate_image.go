package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type validateImageHandler struct {
	rclientFactory engineapi.RegistryClientFactory
	ivCache        imageverifycache.Client
	configuration  config.Configuration
	images         []apiutils.ImageInfo
	client         engineapi.Client
	isCluster      bool
}

func NewValidateImageHandler(
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	configuration config.Configuration,
	rclientFactory engineapi.RegistryClientFactory,
	ivCache imageverifycache.Client,
	client engineapi.Client,
	isCluster bool,
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
	return validateImageHandler{
		rclientFactory: rclientFactory,
		ivCache:        ivCache,
		configuration:  configuration,
		images:         ruleImages,
		client:         client,
		isCluster:      isCluster,
	}, nil
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
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesExceptionWithContext(ctx, h.client, exceptions, policyContext, h.isCluster, logger)
	if len(matchedExceptions) > 0 {
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

	// Resolve variable placeholders (for example a ConfigMap-backed public
	// key) the same way the mutating path does, before using the rule for
	// matching or verification: otherwise a rule that verifies fine at
	// mutate time would see the literal, unresolved placeholder here and
	// fail closed for the wrong reason on every cold-cache admission and
	// background scan.
	ruleCopy, err := internal.SubstituteRuleVariables(rule, policyContext.JSONContext(), logger)
	if err != nil {
		return resource, handlers.WithResponses(
			engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to substitute variables", err, rule.ReportProperties),
		)
	}

	newResource := policyContext.NewResource()
	resourceNamespace := newResource.GetNamespace()
	// Get imagePullSecrets from the first image (all images from same resource share the same secrets)
	var imagePullSecrets []string
	if len(h.images) > 0 {
		imagePullSecrets = h.images[0].ImagePullSecrets
	}

	skippedImages := make([]string, 0)
	passedImages := make([]string, 0)
	for _, v := range ruleCopy.VerifyImages {
		imageVerify := v.Convert()
		// GetMatchingImages appends an image once per matching verifyImages
		// entry, so h.images holds duplicates whenever entries overlap. Each
		// verification is registry and signature work now, so do each distinct
		// image once per entry. The key includes the resource pointer so two
		// containers sharing one image are still reported separately.
		verified := make(map[string]struct{}, len(h.images))
		for _, imageInfo := range h.images {
			image := imageInfo.String()
			key := imageInfo.Pointer + "\x00" + image
			if _, done := verified[key]; done {
				continue
			}
			verified[key] = struct{}{}

			if !engineutils.ImageMatches(image, imageVerify.ImageReferences) {
				logger.V(4).Info("image does not match, skipping", "image", image, "imageReferences", imageVerify.ImageReferences)
				continue
			}

			logger.V(4).Info("validating image", "image", image)
			status, err := h.validateImage(ctx, logger, policyContext, resourceNamespace, imagePullSecrets, *ruleCopy, *imageVerify, imageInfo)
			switch status {
			case engineapi.RuleStatusFail:
				return resource, handlers.WithFail(rule, engineapi.ImageVerify, err.Error())
			case engineapi.RuleStatusError:
				return resource, handlers.WithError(rule, engineapi.ImageVerify, "failed to verify image", err)
			case engineapi.RuleStatusSkip:
				skippedImages = append(skippedImages, image)
			case engineapi.RuleStatusPass:
				passedImages = append(passedImages, image)
			default:
				// required is false: nothing was checked for this image, mirroring
				// pre-existing opt-out semantics for verifyImages.required=false.
			}
		}
	}

	logger.V(4).Info("validated image", "rule", rule.Name)
	if len(passedImages) > 0 || len(passedImages)+len(skippedImages) == 0 {
		if len(skippedImages) > 0 {
			return resource, handlers.WithPass(rule, engineapi.ImageVerify, strings.Join(append([]string{"image verified, skipped images:"}, skippedImages...), " "))
		}
		return resource, handlers.WithPass(rule, engineapi.ImageVerify, "image verified")
	} else {
		return resource, handlers.WithSkip(rule, engineapi.ImageVerify, strings.Join(append([]string{"image skipped, skipped images:"}, skippedImages...), " "))
	}
}

// validateImage independently re-derives the verification verdict for a
// single image instead of trusting metadata left on the resource by a prior
// stage. The validating webhook verifies images independently rather than
// trusting prior-stage metadata: nothing about this decision is read from an
// annotation the admission submitter could have written.
func (h validateImageHandler) validateImage(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resourceNamespace string,
	imagePullSecrets []string,
	rule kyvernov1.Rule,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
) (engineapi.RuleStatus, error) {
	image := imageInfo.String()

	// This fail-closed check runs before anything else and is preserved
	// verbatim: a verifyDigest rule against an unpinned image always fails,
	// regardless of what else the rule configures.
	if imageVerify.VerifyDigest && imageInfo.Digest == "" {
		logger.V(2).Info("missing digest", "image", image)
		return engineapi.RuleStatusFail, fmt.Errorf("missing digest for %s", image)
	}

	if !imageVerify.Required || policyContext.NewResource().Object == nil {
		// required is false: nothing further is enforced at validation time.
		return "", nil
	}

	if len(imageVerify.Attestors) == 0 && len(imageVerify.Attestations) == 0 {
		// required is set but nothing is configured to verify against, so no image can satisfy it.
		return engineapi.RuleStatusFail, fmt.Errorf("rule requires image verification but configures no attestors or attestations: %s", image)
	}

	if engineutils.ImageMatches(image, imageVerify.SkipImageReferences) {
		// verifyImage skips this reference, but it needs a registry client to
		// get there. Decide it here so a skipped image is not reported as an
		// error when credentials are unavailable.
		return engineapi.RuleStatusSkip, nil
	}

	if h.rclientFactory == nil {
		// The factory is an optional engine dependency; verification here needs
		// registry access, so report a rule error rather than panicking.
		return engineapi.RuleStatusError, fmt.Errorf("no registry client configured to verify %s", image)
	}

	rclient, err := h.rclientFactory.GetClient(ctx, imageVerify.ImageRegistryCredentials, resourceNamespace, imagePullSecrets)
	if err != nil {
		return engineapi.RuleStatusError, fmt.Errorf("failed to fetch registry credentials for %s: %w", image, err)
	}
	if rclient == nil {
		// A factory with no global client returns it unchanged when the rule
		// declares no credentials, so a non-nil factory can still yield none.
		return engineapi.RuleStatusError, fmt.Errorf("no registry client configured to verify %s", image)
	}

	// The patches produced here are discarded: the validating webhook cannot
	// mutate the admitted object, it can only decide pass/fail based on its
	// own re-verification.
	ivm := &engineapi.ImageVerificationMetadata{}
	iv := internal.NewImageVerifier(logger, rclient, h.ivCache, policyContext, rule, ivm)
	_, responses := iv.Verify(ctx, imageVerify, []apiutils.ImageInfo{imageInfo}, h.configuration)
	if len(responses) == 0 {
		return engineapi.RuleStatusSkip, nil
	}
	resp := responses[0]
	switch resp.Status() {
	case engineapi.RuleStatusPass:
		return engineapi.RuleStatusPass, nil
	case engineapi.RuleStatusSkip:
		return engineapi.RuleStatusSkip, nil
	case engineapi.RuleStatusError:
		return engineapi.RuleStatusError, errors.New(resp.Message())
	default:
		return engineapi.RuleStatusFail, fmt.Errorf("unverified image %s: %s", image, resp.Message())
	}
}
