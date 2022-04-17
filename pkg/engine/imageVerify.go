package engine

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/kyverno/go-wildcard"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineUtils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
	imageUtils "github.com/kyverno/kyverno/pkg/utils/image"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

func VerifyAndPatchImages(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	images := policyContext.JSONContext.ImageInfo()

	policy := policyContext.Policy
	patchedResource := policyContext.NewResource
	logger := log.Log.WithName("EngineVerifyImages").WithValues("policy", policy.GetName(),
		"kind", patchedResource.GetKind(), "namespace", patchedResource.GetNamespace(), "name", patchedResource.GetName())

	startTime := time.Now()
	defer func() {
		buildResponse(policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "rulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	// update image registry secrets
	if len(registryclient.Secrets) > 0 {
		logger.V(4).Info("updating registry credentials", "secrets", registryclient.Secrets)
		if err := registryclient.UpdateKeychain(); err != nil {
			logger.Error(err, "failed to update image pull secrets")
		}
	}

	rules := autogen.ComputeRules(policyContext.Policy)
	for i := range rules {
		rule := &rules[i]
		if len(rule.VerifyImages) == 0 {
			continue
		}

		if !matches(logger, rule, policyContext) {
			continue
		}

		policyContext.JSONContext.Restore()

		if err := LoadContext(logger, rule.Context, policyContext, rule.Name); err != nil {
			appendError(resp, rule, fmt.Sprintf("failed to load context: %s", err.Error()), response.RuleStatusError)
			continue
		}

		ruleImages := images
		var err error
		if rule.ImageExtractors != nil {
			if ruleImages, err = policyContext.JSONContext.GenerateCustomImageInfo(&policyContext.NewResource, rule.ImageExtractors); err != nil {
				appendError(resp, rule, fmt.Sprintf("failed to extract images: %s", err.Error()), response.RuleStatusError)
				continue
			}
		}

		if ruleImages == nil {
			continue
		}

		ruleCopy, err := substituteVariables(rule, policyContext.JSONContext, logger)
		if err != nil {
			appendError(resp, rule, fmt.Sprintf("failed to substitute variables: %s", err.Error()), response.RuleStatusError)
			continue
		}

		iv := &imageVerifier{
			logger:        logger,
			policyContext: policyContext,
			rule:          ruleCopy,
			resp:          resp,
		}

		for _, imageVerify := range ruleCopy.VerifyImages {
			iv.verify(imageVerify, ruleImages)
		}
	}

	return
}

func appendError(resp *response.EngineResponse, rule *v1.Rule, msg string, status response.RuleStatus) {
	rr := ruleResponse(rule, response.ImageVerify, msg, status)
	resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *rr)
	incrementErrorCount(resp)
}

func substituteVariables(rule *v1.Rule, ctx context.EvalInterface, logger logr.Logger) (*v1.Rule, error) {

	// remove attestations as variables are not substituted in them
	ruleCopy := rule.DeepCopy()
	for _, iv := range ruleCopy.VerifyImages {
		iv.Attestations = nil
	}

	var err error
	*ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, *ruleCopy)
	if err != nil {
		return nil, err
	}

	// replace attestations
	for i := range rule.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = rule.VerifyImages[i].Attestations
	}

	return ruleCopy, nil
}

type imageVerifier struct {
	logger        logr.Logger
	policyContext *PolicyContext
	rule          *v1.Rule
	resp          *response.EngineResponse
}

func (iv *imageVerifier) verify(imageVerify *v1.ImageVerification, images map[string]map[string]imageUtils.ImageInfo) {
	// for backward compatibility
	imageVerify = imageVerify.Convert()

	for _, infoMap := range images {
		for _, imageInfo := range infoMap {
			path := imageInfo.Pointer
			image := imageInfo.String()
			jmespath := engineUtils.JsonPointerToJMESPath(path)
			changed, err := iv.policyContext.JSONContext.HasChanged(jmespath)
			if err == nil && !changed {
				iv.logger.V(4).Info("no change in image, skipping check", "image", image)
				continue
			}

			if !imageMatches(image, imageVerify.ImageReferences) {
				iv.logger.V(4).Info("image does not match pattern", "image", image, "patterns", imageVerify.ImageReferences)
				continue
			}

			var ruleResp *response.RuleResponse
			if len(imageVerify.Attestations) == 0 {
				var digest string
				ruleResp, digest = iv.verifySignatures(imageVerify, imageInfo)
				if ruleResp.Status == response.RuleStatusPass {
					iv.patchDigest(path, imageInfo, digest, ruleResp)
				}
			} else {
				ruleResp = iv.attestImage(imageVerify, imageInfo)
			}

			iv.resp.PolicyResponse.Rules = append(iv.resp.PolicyResponse.Rules, *ruleResp)
			incrementAppliedCount(iv.resp)
		}
	}
}

func imageMatches(image string, imagePatterns []string) bool {
	for _, imagePattern := range imagePatterns {
		if wildcard.Match(imagePattern, image) {
			return true
		}
	}

	return false
}

func (iv *imageVerifier) verifySignatures(imageVerify *v1.ImageVerification, imageInfo imageUtils.ImageInfo) (*response.RuleResponse, string) {
	image := imageInfo.String()
	iv.logger.Info("verifying image", "image", image, "attestors", len(imageVerify.Attestors), "attestations", len(imageVerify.Attestations))

	var digest string
	for i, attestorSet := range imageVerify.Attestors {
		var err error
		path := fmt.Sprintf(".attestors[%d]", i)
		digest, err = iv.verifyAttestorSet(attestorSet, imageVerify, image, path)
		if err != nil {
			iv.logger.Error(err, "failed to verify signature", "attestorSet", attestorSet)
			msg := fmt.Sprintf("failed to verify signature for %s: %s", image, err.Error())
			return ruleResponse(iv.rule, response.ImageVerify, msg, response.RuleStatusFail), ""
		}
	}

	msg := fmt.Sprintf("verified image signatures for %s", image)
	return ruleResponse(iv.rule, response.ImageVerify, msg, response.RuleStatusPass), digest
}

func (iv *imageVerifier) verifyAttestorSet(attestorSet *v1.AttestorSet, imageVerify *v1.ImageVerification, image, path string) (string, error) {
	var errorList []error
	verifiedCount := 0
	requiredCount := getRequiredCount(attestorSet)
	for i, a := range attestorSet.Entries {
		var digest string
		var entryError error
		attestorPath := fmt.Sprintf("%s.entries[%d]", path, i)

		if a.Attestor != nil {
			nestedAttestorSet, err := v1.AttestorSetUnmarshal(a.Attestor)
			if err != nil {
				entryError = errors.Wrapf(err, "failed to unmarshal nested attestor %s", attestorPath)
			} else {
				attestorPath += ".attestor"
				digest, entryError = iv.verifyAttestorSet(nestedAttestorSet, imageVerify, image, attestorPath)
			}
		} else {
			opts, subPath := iv.buildOptionsAndPath(a, imageVerify, image)
			digest, entryError = cosign.VerifySignature(*opts)
			if entryError != nil {
				entryError = fmt.Errorf("%s: %s", attestorPath+subPath, entryError.Error())
			}
		}

		if entryError == nil {
			verifiedCount++
			if verifiedCount >= requiredCount {
				iv.logger.V(2).Info("image verification succeeded", "verifiedCount", verifiedCount, "requiredCount", requiredCount)
				return digest, nil
			}
		} else {
			errorList = append(errorList, entryError)
		}
	}

	iv.logger.Info("image verification failed", "verifiedCount", verifiedCount, "requiredCount", requiredCount, "errors", errorList)
	err := engineUtils.CombineErrors(errorList)
	return "", err
}

func getRequiredCount(as *v1.AttestorSet) int {
	if as.Count == nil || *as.Count == 0 {
		return len(as.Entries)
	}

	return *as.Count
}

func (iv *imageVerifier) buildOptionsAndPath(attestor *v1.Attestor, imageVerify *v1.ImageVerification, image string) (*cosign.Options, string) {
	path := ""
	opts := &cosign.Options{
		ImageRef:    image,
		Repository:  imageVerify.Repository,
		Annotations: imageVerify.Annotations,
		Log:         iv.logger,
	}

	if attestor.StaticKey != nil {
		path = path + ".staticKey"
		opts.Key = attestor.StaticKey.Key
		opts.Roots = []byte(attestor.StaticKey.Roots)
	} else if attestor.Keyless != nil {
		path = path + ".keyless"
		opts.Roots = []byte(attestor.Keyless.Roots)
		opts.Issuer = attestor.Keyless.Issuer
		opts.Subject = attestor.Keyless.Subject
		opts.AdditionalExtensions = attestor.Keyless.AdditionalExtensions
	}

	if attestor.Repository != "" {
		opts.Repository = attestor.Repository
	}

	if attestor.Annotations != nil {
		opts.Annotations = attestor.Annotations
	}

	return opts, path
}

func (iv *imageVerifier) patchDigest(path string, imageInfo imageUtils.ImageInfo, digest string, ruleResp *response.RuleResponse) {
	if imageInfo.Digest == "" {
		patch, err := makeAddDigestPatch(path, imageInfo, digest)
		if err != nil {
			iv.logger.Error(err, "failed to patch image with digest", "image", imageInfo.String(), "jsonPath", path)
		} else {
			iv.logger.V(4).Info("patching verified image with digest", "patch", string(patch))
			ruleResp.Patches = [][]byte{patch}
		}
	}
}

func makeAddDigestPatch(path string, imageInfo imageUtils.ImageInfo, digest string) ([]byte, error) {
	var patch = make(map[string]interface{})
	patch["op"] = "replace"
	patch["path"] = path
	patch["value"] = imageInfo.String() + "@" + digest
	return json.Marshal(patch)
}

func (iv *imageVerifier) attestImage(imageVerify *v1.ImageVerification, imageInfo imageUtils.ImageInfo) *response.RuleResponse {
	image := imageInfo.String()
	start := time.Now()

	statements, err := cosign.FetchAttestations(image, imageVerify, iv.logger)
	if err != nil {
		iv.logger.Info("failed to fetch attestations", "image", image, "error", err, "duration", time.Since(start).Seconds())
		return ruleError(iv.rule, response.ImageVerify, fmt.Sprintf("failed to fetch attestations for %s", image), err)
	}

	iv.logger.V(4).Info("received attestations", "statements", statements)
	statementsByPredicate := buildStatementMap(statements)

	for _, ac := range imageVerify.Attestations {
		statements := statementsByPredicate[ac.PredicateType]
		if statements == nil {
			msg := fmt.Sprintf("predicate type %s not found", ac.PredicateType)
			return ruleResponse(iv.rule, response.ImageVerify, msg, response.RuleStatusFail)
		}

		for _, s := range statements {
			val, err := iv.checkAttestations(ac, s, imageInfo)
			if err != nil {
				return ruleError(iv.rule, response.ImageVerify, "failed to check attestation", err)
			}

			if !val {
				msg := fmt.Sprintf("attestation checks failed for %s and predicate %s", imageInfo.String(), ac.PredicateType)
				return ruleResponse(iv.rule, response.ImageVerify, msg, response.RuleStatusFail)
			}
		}
	}

	msg := fmt.Sprintf("attestation checks passed for %s", imageInfo.String())
	iv.logger.V(2).Info(msg)
	return ruleResponse(iv.rule, response.ImageVerify, msg, response.RuleStatusPass)
}

func buildStatementMap(statements []map[string]interface{}) map[string][]map[string]interface{} {
	results := map[string][]map[string]interface{}{}
	for _, s := range statements {
		predicateType := s["predicateType"].(string)
		if results[predicateType] != nil {
			results[predicateType] = append(results[predicateType], s)
		} else {
			results[predicateType] = []map[string]interface{}{s}
		}
	}

	return results
}

func (iv *imageVerifier) checkAttestations(a *v1.Attestation, s map[string]interface{}, img imageUtils.ImageInfo) (bool, error) {
	if len(a.Conditions) == 0 {
		return true, nil
	}

	iv.policyContext.JSONContext.Checkpoint()
	defer iv.policyContext.JSONContext.Restore()

	predicate, ok := s["predicate"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("failed to extract predicate from statement: %v", s)
	}

	if err := context.AddJSONObject(iv.policyContext.JSONContext, predicate); err != nil {
		return false, errors.Wrapf(err, fmt.Sprintf("failed to add Statement to the context %v", s))
	}

	if err := iv.policyContext.JSONContext.AddImageInfo(img); err != nil {
		return false, errors.Wrapf(err, fmt.Sprintf("failed to add image to the context %v", s))
	}

	conditions, err := variables.SubstituteAllInConditions(iv.logger, iv.policyContext.JSONContext, a.Conditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in attestation conditions")
	}

	pass := variables.EvaluateAnyAllConditions(iv.logger, iv.policyContext.JSONContext, conditions)
	return pass, nil
}
