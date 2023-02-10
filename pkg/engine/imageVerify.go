package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/tracing"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"go.opentelemetry.io/otel/trace"
)

func (e *engine) verifyAndPatchImages(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (*engineapi.EngineResponse, *engineapi.ImageVerificationMetadata) {
	policy := policyContext.Policy()
	resp := engineapi.NewEngineResponse(policy)
	startTime := time.Now()
	defer func() {
		internal.BuildResponse(policyContext, resp, startTime)
		logger.V(4).Info("processed image verification rules",
			"time", resp.PolicyResponse.ProcessingTime.String(),
			"applied", resp.PolicyResponse.RulesAppliedCount, "successful", resp.IsSuccessful())
	}()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	ivm := &engineapi.ImageVerificationMetadata{}
	rules := autogen.ComputeRules(policyContext.Policy())
	applyRules := policy.GetSpec().GetApplyRules()

	for i := range rules {
		rule := &rules[i]

		tracing.ChildSpan(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) {
				if len(rule.VerifyImages) == 0 {
					return
				}
				startTime := time.Now()
				logger := internal.LoggerWithRule(logger, rules[i])
				kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
				subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(e.client, kindsInPolicy, policyContext)

				if !matches(logger, rule, policyContext, subresourceGVKToAPIResource, e.configuration) {
					return
				}

				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(logger, engineapi.ImageVerify, e.exceptionSelector, policyContext, rule, subresourceGVKToAPIResource, e.configuration)
				if ruleResp != nil {
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
					return
				}

				logger.V(3).Info("processing image verification rule")

				ruleImages, imageRefs, err := e.extractMatchingImages(policyContext, rule)
				if err != nil {
					internal.AddRuleResponse(
						resp.PolicyResponse,
						internal.RuleError(rule, engineapi.ImageVerify, "failed to extract images", err),
						startTime,
					)
					return
				}
				if len(ruleImages) == 0 {
					internal.AddRuleResponse(
						resp.PolicyResponse,
						internal.RuleSkip(
							rule,
							engineapi.ImageVerify,
							fmt.Sprintf("skip run verification as image in resource not found in imageRefs '%s'", imageRefs),
						),
						startTime,
					)
					return
				}
				policyContext.JSONContext().Restore()
				if err := internal.LoadContext(ctx, e, policyContext, *rule); err != nil {
					internal.AddRuleResponse(
						resp.PolicyResponse,
						internal.RuleError(rule, engineapi.ImageVerify, "failed to load context", err),
						startTime,
					)
					return
				}
				ruleCopy, err := substituteVariables(rule, policyContext.JSONContext(), logger)
				if err != nil {
					internal.AddRuleResponse(
						resp.PolicyResponse,
						internal.RuleError(rule, engineapi.ImageVerify, "failed to substitute variables", err),
						startTime,
					)
					return
				}
				iv := internal.NewImageVerifier(
					logger,
					e.rclient,
					policyContext,
					ruleCopy,
					ivm,
				)
				for _, imageVerify := range ruleCopy.VerifyImages {
					for _, r := range iv.Verify(ctx, imageVerify, ruleImages, e.configuration) {
						internal.AddRuleResponse(resp.PolicyResponse, r, startTime)
					}
				}
			},
		)

		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.RulesAppliedCount > 0 {
			break
		}
	}

	return resp, ivm
}

func getMatchingImages(images map[string]map[string]apiutils.ImageInfo, rule *kyvernov1.Rule) ([]apiutils.ImageInfo, string) {
	imageInfos := []apiutils.ImageInfo{}
	imageRefs := []string{}
	for _, infoMap := range images {
		for _, imageInfo := range infoMap {
			image := imageInfo.String()
			for _, verifyImage := range rule.VerifyImages {
				verifyImage = *verifyImage.Convert()
				imageRefs = append(imageRefs, verifyImage.ImageReferences...)
				if imageMatches(image, verifyImage.ImageReferences) {
					imageInfos = append(imageInfos, imageInfo)
				}
			}
		}
	}
	return imageInfos, strings.Join(imageRefs, ",")
}

func imageMatches(image string, imagePatterns []string) bool {
	for _, imagePattern := range imagePatterns {
		if wildcard.Match(imagePattern, image) {
			return true
		}
	}

	return false
}

func (e *engine) extractMatchingImages(policyContext engineapi.PolicyContext, rule *kyvernov1.Rule) ([]apiutils.ImageInfo, string, error) {
	var (
		images map[string]map[string]apiutils.ImageInfo
		err    error
	)
	newResource := policyContext.NewResource()
	images = policyContext.JSONContext().ImageInfo()
	if rule.ImageExtractors != nil {
		images, err = policyContext.JSONContext().GenerateCustomImageInfo(&newResource, rule.ImageExtractors, e.configuration)
		if err != nil {
			// if we get an error while generating custom images from image extractors,
			// don't check for matching images in imageExtractors
			return nil, "", err
		}
	}
	matchingImages, imageRefs := getMatchingImages(images, rule)
	return matchingImages, imageRefs, nil
}

func substituteVariables(rule *kyvernov1.Rule, ctx enginecontext.EvalInterface, logger logr.Logger) (*kyvernov1.Rule, error) {
	// remove attestations as variables are not substituted in them
	ruleCopy := *rule.DeepCopy()
	for i := range ruleCopy.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = nil
	}

	var err error
	ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, ruleCopy)
	if err != nil {
		return nil, err
	}

	// replace attestations
	for i := range rule.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = rule.VerifyImages[i].Attestations
	}

	return &ruleCopy, nil
}
