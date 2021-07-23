package engine

import (
	"encoding/json"
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

func VerifyAndPatchImages(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	images := policyContext.JSONContext.ImageInfo()
	if images == nil {
		return
	}

	policy := policyContext.Policy
	patchedResource := policyContext.NewResource
	logger := log.Log.WithName("EngineVerifyImages").WithValues("policy", policy.Name,
		"kind", patchedResource.GetKind(), "namespace", patchedResource.GetNamespace(), "name", patchedResource.GetName())

	if ManagedPodResource(policy, patchedResource) {
		logger.V(4).Info("images for resources managed by workload controllers are already verified", "policy", policy.GetName())
		resp.PatchedResource = patchedResource
		return
	}

	startTime := time.Now()
	defer func() {
		buildResponse(logger, policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "rulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	for i := range policyContext.Policy.Spec.Rules {
		rule := policyContext.Policy.Spec.Rules[i]
		if len(rule.VerifyImages) == 0 {
			continue
		}

		if !matches(logger, rule, policyContext) {
			continue
		}

		policyContext.JSONContext.Restore()
		for _, imageVerify := range rule.VerifyImages {
			verifyAndPatchImages(logger, policyContext, &rule, imageVerify, images.Containers, resp)
			verifyAndPatchImages(logger, policyContext, &rule, imageVerify, images.InitContainers, resp)
		}
	}

	return
}

func verifyAndPatchImages(logger logr.Logger, policyContext *PolicyContext, rule *v1.Rule, imageVerify *v1.ImageVerification, images map[string]*context.ImageInfo, resp *response.EngineResponse) {
	imagePattern := imageVerify.Image
	key := imageVerify.Key

	for _, imageInfo := range images {
		image := imageInfo.String()
		jmespath := utils.JsonPointerToJMESPath(imageInfo.JSONPointer)
		changed, err := policyContext.JSONContext.HasChanged(jmespath)
		if err == nil && !changed {
			logger.V(4).Info("no change in image, skipping check", "image", image)
			continue
		}

		if !wildcard.Match(imagePattern, image) {
			logger.V(4).Info("image does not match pattern", "image", image, "pattern", imagePattern)
			continue
		}

		logger.Info("verifying image", "image", image)
		incrementAppliedCount(resp)

		ruleResp := response.RuleResponse{
			Name: rule.Name,
			Type: utils.Validation.String(),
		}

		start := time.Now()
		digest, err := cosign.Verify(image, []byte(key), logger)
		if err != nil {
			logger.Info("failed to verify image", "image", image, "key", key, "error", err, "duration", time.Since(start).Seconds())
			ruleResp.Success = false
			ruleResp.Message = fmt.Sprintf("image verification failed for %s: %v", image, err)
		} else {
			logger.V(3).Info("verified image", "image", image, "digest", digest, "duration", time.Since(start).Seconds())
			ruleResp.Success = true
			ruleResp.Message = fmt.Sprintf("image %s verified", image)

			// add digest to image
			if imageInfo.Digest == "" {
				patch, err := makeAddDigestPatch(imageInfo, digest)
				if err != nil {
					logger.Error(err, "failed to patch image with digest", "image", imageInfo.String(), "jsonPath", imageInfo.JSONPointer)
				} else {
					logger.V(4).Info("patching verified image with digest", "patch", string(patch))
					ruleResp.Patches = [][]byte{patch}
				}
			}
		}

		resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResp)
	}
}

func makeAddDigestPatch(imageInfo *context.ImageInfo, digest string) ([]byte, error) {
	var patch = make(map[string]interface{})
	patch["op"] = "replace"
	patch["path"] = imageInfo.JSONPointer
	patch["value"] = imageInfo.String() + "@" + digest
	return json.Marshal(patch)
}
