package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/pkg/errors"

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

	startTime := time.Now()
	defer func() {
		buildResponse(policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "rulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	// update image registry secrets
	if len(cosign.Secrets) > 0 {
		logger.V(4).Info("updating registry credentials", "secrets", cosign.Secrets)
		if err := cosign.UpdateKeychain(); err != nil {
			logger.Error(err, "failed to update image pull secrets")
		}
	}

	for i := range policyContext.Policy.Spec.Rules {
		rule := &policyContext.Policy.Spec.Rules[i]
		if len(rule.VerifyImages) == 0 {
			continue
		}

		if !matches(logger, rule, policyContext) {
			continue
		}

		policyContext.JSONContext.Restore()

		iv := &imageVerifier{
			logger:        logger,
			policyContext: policyContext,
			rule:          rule,
			resp:          resp,
		}

		for _, imageVerify := range rule.VerifyImages {
			iv.verify(imageVerify, images.Containers)
			iv.verify(imageVerify, images.InitContainers)
		}
	}

	return
}

type imageVerifier struct {
	logger        logr.Logger
	policyContext *PolicyContext
	rule          *v1.Rule
	resp          *response.EngineResponse
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
			if ruleResp.Status == response.RuleStatusPass {
				iv.patchDigest(imageInfo, digest, ruleResp)
			}
		} else {
			ruleResp = iv.attestImage(repository, key, imageInfo, imageVerify.Attestations)
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
		ruleResp.Status = response.RuleStatusFail
		ruleResp.Message = fmt.Sprintf("image signature verification failed for %s: %v", image, err)
		return ruleResp, ""
	}

	ruleResp.Status = response.RuleStatusPass
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

func (iv *imageVerifier) attestImage(repository, key string, imageInfo *context.ImageInfo, attestationChecks []*v1.Attestation) *response.RuleResponse {
	image := imageInfo.String()

	start := time.Now()
	statements, err := cosign.FetchAttestations(image, []byte(key), repository)
	if err != nil {
		iv.logger.Info("failed to fetch attestations", "image", image, "error", err, "duration", time.Since(start).Seconds())
		return ruleError(iv.rule, utils.ImageVerify, fmt.Sprintf("failed to fetch attestations for %s", image), err)
	}

	iv.logger.V(3).Info("received attested statements", "statements", statements)

	for _, ac := range attestationChecks {
		for _, s := range statements {
			predicateType := s["predicateType"]
			if ac.PredicateType == predicateType {
				val, err := iv.checkAttestations(ac, s, imageInfo)
				if err != nil {
					return ruleError(iv.rule, utils.ImageVerify, "error while checking attestation", err)
				}

				if !val {
					msg := fmt.Sprintf("attestation checks failed for %s and predicate %s", imageInfo.String(), predicateType)
					iv.logger.Info(msg)
					return ruleResponse(iv.rule, utils.ImageVerify, msg, response.RuleStatusFail)
				}
			}
		}
	}

	msg := fmt.Sprintf("attestation checks passed for %s", imageInfo.String())
	iv.logger.V(2).Info(msg)
	return ruleResponse(iv.rule, utils.ImageVerify, msg, response.RuleStatusPass)
}

func (iv *imageVerifier) checkAttestations(a *v1.Attestation, s map[string]interface{}, img *context.ImageInfo) (bool, error) {
	if len(a.Conditions) == 0 {
		return true, nil
	}

	iv.policyContext.JSONContext.Checkpoint()
	defer iv.policyContext.JSONContext.Restore()

	predicate, ok := s["predicate"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("failed to extract predicate from statement: %v", s)
	}

	if err := iv.policyContext.JSONContext.AddJSONObject(predicate); err != nil {
		return false, errors.Wrapf(err, fmt.Sprintf("failed to add Statement to the context %v", s))
	}

	imgMap := map[string]interface{}{
		"image": map[string]interface{}{
			"image":    img.String(),
			"registry": img.Registry,
			"path":     img.Path,
			"name":     img.Name,
			"tag":      img.Tag,
			"digest":   img.Digest,
		},
	}

	if err := iv.policyContext.JSONContext.AddJSONObject(imgMap); err != nil {
		return false, errors.Wrapf(err, fmt.Sprintf("failed to add image to the context %v", s))
	}

	conditions, err := variables.SubstituteAllInConditions(iv.logger, iv.policyContext.JSONContext, a.Conditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in attestation conditions")
	}

	pass := variables.EvaluateAnyAllConditions(iv.logger, iv.policyContext.JSONContext, conditions)
	return pass, nil
}
