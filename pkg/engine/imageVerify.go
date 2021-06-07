package engine

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/minio/minio/pkg/wildcard"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

func verifyImages(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	images := policyContext.JSONContext.ImageInfo()
	if images == nil {
		return
	}

	policy := policyContext.Policy
	patchedResource := policyContext.NewResource
	logger := log.Log.WithName("EngineVerifyImages").WithValues("policy", policy.Name,
		"kind", patchedResource.GetKind(), "namespace", patchedResource.GetNamespace(), "name", patchedResource.GetName())

	startTime := time.Now()
	defer func() {
		buildResponse(logger, policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	for _, rule := range policyContext.Policy.Spec.Rules {
		if len(rule.VerifyImages) == 0 {
			continue
		}

		if !matches(logger, rule, policyContext) {
			continue
		}

		for _, imageVerify := range rule.VerifyImages {
			verifyImageInfos(logger, &rule, imageVerify, images.Containers, resp)
			verifyImageInfos(logger, &rule, imageVerify, images.InitContainers, resp)
		}
	}

	return
}

func verifyImageInfos(logger logr.Logger, rule *v1.Rule, imageVerify *v1.ImageVerification, images map[string]*context.ImageInfo, resp *response.EngineResponse) {
	imagePattern := imageVerify.Image
	key := imageVerify.Key

	for _, imageInfo := range images {
		image := imageInfo.String()
		if wildcard.Match(imagePattern, image) {
			logger.Info("verifying image", "image", image)
			incrementAppliedCount(resp)

			ruleResp := response.RuleResponse{
				Name:    rule.Name,
				Type:    utils.Validation.String(),
				Success: true,
				Message: fmt.Sprintf("image %s verified", image),
			}

			// TODO - use digest to mutate the image
			_, err := cosign.Verify(image, []byte(key))
			if err != nil {
				ruleResp.Success = false
				ruleResp.Message = fmt.Sprintf("image verification failed for %s: %v", image, err)
			}

			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResp)
		}
	}
}
