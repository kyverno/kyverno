package internal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"gomodules.xyz/jsonpatch/v2"
)

func ExpandStaticKeys(attestorSet kyvernov1.AttestorSet) kyvernov1.AttestorSet {
	entries := make([]kyvernov1.Attestor, 0, len(attestorSet.Entries))
	for _, e := range attestorSet.Entries {
		if e.Keys != nil {
			keys := splitPEM(e.Keys.PublicKeys)
			if len(keys) > 1 {
				moreEntries := createStaticKeyAttestors(keys, e)
				entries = append(entries, moreEntries...)
				continue
			}
		}
		entries = append(entries, e)
	}
	return kyvernov1.AttestorSet{
		Count:   attestorSet.Count,
		Entries: entries,
	}
}

func EvaluateConditions(
	conditions []kyvernov1.AnyAllConditions,
	ctx enginecontext.Interface,
	s map[string]any,
	log logr.Logger,
) (bool, string, error) {
	predicate, ok := s["predicate"].(map[string]any)
	if !ok {
		return false, "", fmt.Errorf("failed to extract predicate from statement: %v", s)
	}
	if err := enginecontext.AddJSONObject(ctx, predicate); err != nil {
		return false, "", fmt.Errorf("failed to add Statement to the context %v: %w", s, err)
	}
	c, err := variables.SubstituteAllInConditions(log, ctx, conditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in attestation conditions: %w", err)
	}
	return variables.EvaluateAnyAllConditionsWithContext(log, ctx, c, "attestation.conditions")
}

func matchReferences(imageReferences []string, image string) bool {
	for _, imageRef := range imageReferences {
		if wildcard.Match(imageRef, image) {
			return true
		}
	}
	return false
}

func ruleStatusToImageVerificationStatus(ruleStatus engineapi.RuleStatus) engineapi.ImageVerificationMetadataStatus {
	var imageVerificationResult engineapi.ImageVerificationMetadataStatus
	switch ruleStatus {
	case engineapi.RuleStatusPass:
		imageVerificationResult = engineapi.ImageVerificationPass
	case engineapi.RuleStatusSkip:
		imageVerificationResult = engineapi.ImageVerificationSkip
	case engineapi.RuleStatusWarn:
		imageVerificationResult = engineapi.ImageVerificationSkip
	case engineapi.RuleStatusFail:
		imageVerificationResult = engineapi.ImageVerificationFail
	default:
		imageVerificationResult = engineapi.ImageVerificationFail
	}
	return imageVerificationResult
}

func splitPEM(pem string) []string {
	keys := strings.SplitAfter(pem, "-----END PUBLIC KEY-----")
	if len(keys) < 1 {
		return keys
	}
	return keys[0 : len(keys)-1]
}

func createStaticKeyAttestors(keys []string, base kyvernov1.Attestor) []kyvernov1.Attestor {
	attestors := make([]kyvernov1.Attestor, 0, len(keys))
	for _, k := range keys {
		a := base.DeepCopy()
		a.Keys.PublicKeys = k
		attestors = append(attestors, *a)
	}
	return attestors
}

func buildStatementMap(statements []map[string]any) (map[string][]map[string]any, []string, error) {
	results := map[string][]map[string]any{}
	predicateTypes := make([]string, 0, len(statements))
	for _, s := range statements {
		predicateType, ok := s["type"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("statement missing or non-string 'type' field: %v", s)
		}
		if results[predicateType] != nil {
			results[predicateType] = append(results[predicateType], s)
		} else {
			results[predicateType] = []map[string]any{s}
		}
		predicateTypes = append(predicateTypes, predicateType)
	}
	return results, predicateTypes, nil
}

func makeAddDigestPatch(imageInfo apiutils.ImageInfo, digest string) jsonpatch.JsonPatchOperation {
	return jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      imageInfo.Pointer,
		Value:     imageInfo.String() + "@" + digest,
	}
}

func getRawResp(statements []map[string]any) ([]byte, error) {
	for _, statement := range statements {
		predicate, ok := statement["predicate"].(map[string]any)
		if ok {
			rawResp, err := json.Marshal(predicate)
			if err != nil {
				return nil, err
			}
			return rawResp, nil
		}
	}
	return nil, fmt.Errorf("predicate not found in any statement")
}
