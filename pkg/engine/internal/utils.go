package internal

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

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

func buildStatementMap(statements []map[string]any) (map[string][]map[string]any, []string) {
	results := map[string][]map[string]any{}
	predicateTypes := make([]string, 0, len(statements))
	for _, s := range statements {
		predicateType := s["type"].(string)
		if results[predicateType] != nil {
			results[predicateType] = append(results[predicateType], s)
		} else {
			results[predicateType] = []map[string]any{s}
		}
		predicateTypes = append(predicateTypes, predicateType)
	}
	return results, predicateTypes
}

func makeAddDigestPatch(imageInfo apiutils.ImageInfo, digest string) jsonpatch.JsonPatchOperation {
	return jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      imageInfo.Pointer,
		Value:     imageInfo.String() + "@" + digest,
	}
}

var timestampKeys = []string{
	"scanTime",
	"buildStartedOn",
	"buildFinishedOn",
	"startedOn",
	"finishedOn",
	"timestamp",
	"CreatedAt",
	"created",
	"time",
}

func extractStatementTimestamp(statement map[string]any) (time.Time, bool) {
	if predicate, ok := statement["predicate"].(map[string]any); ok {
		if meta, ok := predicate["metadata"].(map[string]any); ok {
			for _, k := range timestampKeys {
				if t, ok := parseTimeValue(meta[k]); ok {
					return t, true
				}
			}
		}
		if runDetails, ok := predicate["runDetails"].(map[string]any); ok {
			if meta, ok := runDetails["metadata"].(map[string]any); ok {
				for _, k := range timestampKeys {
					if t, ok := parseTimeValue(meta[k]); ok {
						return t, true
					}
				}
			}
		}
		if creationInfo, ok := predicate["creationInfo"].(map[string]any); ok {
			if t, ok := parseTimeValue(creationInfo["created"]); ok {
				return t, true
			}
		}
		for _, k := range timestampKeys {
			if t, ok := parseTimeValue(predicate[k]); ok {
				return t, true
			}
		}
	}

	if annotations, ok := statement["annotations"].(map[string]any); ok {
		for _, k := range []string{"org.opencontainers.image.created", "dev.cosignproject.cosign/created"} {
			if t, ok := parseTimeValue(annotations[k]); ok {
				return t, true
			}
		}
	}
	for _, k := range []string{"created", "timestamp", "time"} {
		if t, ok := parseTimeValue(statement[k]); ok {
			return t, true
		}
	}

	return time.Time{}, false
}

func parseTimeValue(val any) (time.Time, bool) {
	s, ok := val.(string)
	if !ok || s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func sortStatementsByTimestamp(statements []map[string]any) {
	sort.SliceStable(statements, func(i, j int) bool {
		ti, oki := extractStatementTimestamp(statements[i])
		tj, okj := extractStatementTimestamp(statements[j])
		if oki && okj {
			return ti.After(tj)
		}
		if oki {
			return true
		}
		return false
	})
}

func getRawResp(statements []map[string]any, attestationType string) ([]byte, error) {
	matching := make([]map[string]any, 0, len(statements))
	for _, s := range statements {
		t, _ := s["type"].(string)
		if attestationType == "" || t == attestationType {
			if _, ok := s["predicate"].(map[string]any); ok {
				matching = append(matching, s)
			}
		}
	}
	if len(matching) == 0 {
		for _, s := range statements {
			if _, ok := s["predicate"].(map[string]any); ok {
				matching = append(matching, s)
				break
			}
		}
	}
	if len(matching) == 0 {
		return nil, fmt.Errorf("predicate not found in any statement")
	}

	sortStatementsByTimestamp(matching)
	predicate := matching[0]["predicate"].(map[string]any)
	return json.Marshal(predicate)
}
