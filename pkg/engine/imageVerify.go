package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/minio/pkg/wildcard"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

		iv := &imageVerifier {
			logger: logger,
			policyContext: policyContext,
			rule: &rule,
			resp: resp,
		}

		for _, imageVerify := range rule.VerifyImages {
			iv.verify(imageVerify, images.Containers)
			iv.verify(imageVerify, images.InitContainers)
		}
	}

	return
}

type imageVerifier struct {
	logger logr.Logger
	policyContext *PolicyContext
	rule *v1.Rule
	resp *response.EngineResponse
}

func (iv *imageVerifier) verify(imageVerify *v1.ImageVerification, images map[string]*context.ImageInfo) {
	imagePattern := imageVerify.Image
	key := imageVerify.Key
	repository := getSignatureRepository(imageVerify)

	for _, imageInfo := range images {
		image := imageInfo.String()
		jmespath := utils.JsonPointerToJMESPath(imageInfo.JSONPointer)
		changed, err := iv.policyContext.JSONContext.HasChanged(jmespath)
		if err == nil && !changed {
			iv.logger.V(4).Info("no change in image, skipping check", "image", image)
			continue
		}

		if !wildcard.Match(imagePattern, image) {
			iv.logger.V(4).Info("image does not match pattern", "image", image, "pattern", imagePattern)
			continue
		}

		var ruleResp *response.RuleResponse
		if len(imageVerify.Attestations) == 0 {
			var digest string
			ruleResp, digest = iv.verifySignature(repository, key, imageInfo)
			if ruleResp.Success {
				iv.patchDigest(imageInfo, digest, ruleResp)
			}
		} else {
			ruleResp = iv.attestImage(repository, key, imageInfo)
		}

		iv.resp.PolicyResponse.Rules = append(iv.resp.PolicyResponse.Rules, *ruleResp)
		incrementAppliedCount(iv.resp)
	}
}

func getSignatureRepository(imageVerify *v1.ImageVerification) string {
	repository := cosign.ImageSignatureRepository
	if imageVerify.Repository != "" {
		repository = imageVerify.Repository
	}

	return repository
}

func (iv *imageVerifier) verifySignature(repository, key string, imageInfo *context.ImageInfo) (*response.RuleResponse, string) {
	image := imageInfo.String()
	iv.logger.Info("verifying image", "image", image)

	ruleResp := &response.RuleResponse{
		Name: iv.rule.Name,
		Type: utils.Validation.String(),
	}

	start := time.Now()
	digest, err := cosign.VerifySignature(image, []byte(key), repository, iv.logger)
	if err != nil {
		iv.logger.Info("failed to verify image signature", "image", image, "error", err, "duration", time.Since(start).Seconds())
		ruleResp.Success = false
		ruleResp.Message = fmt.Sprintf("image signature verification failed for %s: %v", image, err)
		return ruleResp, ""
	}

	ruleResp.Success = true
	ruleResp.Message = fmt.Sprintf("image %s verified", image)
	iv.logger.V(3).Info("verified image", "image", image, "digest", digest, "duration", time.Since(start).Seconds())
	return ruleResp, digest
}

func (iv *imageVerifier) patchDigest(imageInfo *context.ImageInfo, digest string, ruleResp *response.RuleResponse) {
	if imageInfo.Digest == "" {
		patch, err := makeAddDigestPatch(imageInfo, digest)
		if err != nil {
			iv.logger.Error(err, "failed to patch image with digest", "image", imageInfo.String(), "jsonPath", imageInfo.JSONPointer)
		} else {
			iv.logger.V(4).Info("patching verified image with digest", "patch", string(patch))
			ruleResp.Patches = [][]byte{patch}
		}
	}
}

func makeAddDigestPatch(imageInfo *context.ImageInfo, digest string) ([]byte, error) {
	var patch = make(map[string]interface{})
	patch["op"] = "replace"
	patch["path"] = imageInfo.JSONPointer
	patch["value"] = imageInfo.String() + "@" + digest
	return json.Marshal(patch)
}

func (iv *imageVerifier) attestImage(repository, key string, imageInfo *context.ImageInfo) *response.RuleResponse {
	image := imageInfo.String()

	ruleResp := &response.RuleResponse{
		Name: iv.rule.Name,
		Type: utils.Validation.String(),
	}

	start := time.Now()
	inTotoAttestation, err := cosign.FetchAttestations(image, []byte(key), repository)
	if err != nil {
		iv.logger.Info("failed to verify image attestations", "image", image, "error", err, "duration", time.Since(start).Seconds())
		ruleResp.Success = false
		ruleResp.Message = fmt.Sprintf("image attestation failed for %s: %v", image, err)
		return ruleResp
	}

	iv.logger.Info("received attestation", "in-toto-attestation", inTotoAttestation)


	// add to context

	// process any / all conditions
	return ruleResp
}
