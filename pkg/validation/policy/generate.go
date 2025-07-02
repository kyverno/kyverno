package policy

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func immutableGenerateFields(new, old kyvernov1.PolicyInterface) (string, error) {
	if new == nil || old == nil {
		return "", nil
	}

	oldHasSynchronizingRule := hasSynchronizingRule(old.GetSpec().Rules)

	oldRuleHashes, oldGenerationHashes, err := buildHashes(old.GetSpec().Rules, oldHasSynchronizingRule)
	if err != nil {
		return "", err
	}
	newRuleHashes, newGenerationHashes, err := buildHashes(new.GetSpec().Rules, oldHasSynchronizingRule)
	if err != nil {
		return "", err
	}

	if !newGenerationHashes.IsSuperset(oldGenerationHashes) {
		return "changes in the generate rule pattern could result in stale targets", nil
	}

	if !newRuleHashes.IsSuperset(oldRuleHashes) {
		return "", errors.New("changes of immutable fields of a rule spec in a generate rule is disallowed")
	}

	return "", nil
}

func resetMutableFields(rule kyvernov1.Rule, oldHasSynchronizingRule bool) (*kyvernov1.Rule, *kyvernov1.Generation) {
	new := new(kyvernov1.Rule)
	rule.DeepCopyInto(new)
	generation := new.Generation
	new.Generation = nil

	// we allow changing the matching only if the rule has
	// synchronize set to false; otherwise, we risk having
	// stale targets and confusing synchronization behavior
	if !oldHasSynchronizingRule {
		new.MatchResources = kyvernov1.MatchResources{}
	}

	generation.Synchronize = true
	generation.SetData(nil)
	generation.ForEachGeneration = nil
	generation.OrphanDownstreamOnPolicyDelete = true
	generation.GenerateExisting = nil

	return new, generation
}

func buildHashes(rules []kyvernov1.Rule, oldHasSynchronizingRule bool) (ruleHashes sets.Set[string], generationHashes sets.Set[string], _ error) {
	ruleHashes, generationHashes = sets.New[string](), sets.New[string]()

	for _, rule := range rules {
		if !rule.HasGenerate() {
			continue
		}
		r, generation := resetMutableFields(rule, oldHasSynchronizingRule)
		data, err := json.Marshal(generation)
		if err != nil {
			return ruleHashes, generationHashes, fmt.Errorf("failed to create hash from the generate rule %v", err)
		}
		hash := md5.Sum(data) //nolint:gosec
		generationHashes.Insert(hex.EncodeToString(hash[:]))

		data, err = json.Marshal(r)
		if err != nil {
			return ruleHashes, generationHashes, fmt.Errorf("failed to create hash from the generate rule %v", err)
		}
		hash = md5.Sum(data) //nolint:gosec
		ruleHashes.Insert(hex.EncodeToString(hash[:]))
	}
	return ruleHashes, generationHashes, nil
}

func hasSynchronizingRule(rules []kyvernov1.Rule) bool {
	for _, r := range rules {
		if !r.HasGenerate() {
			continue
		}

		if r.Generation.Synchronize {
			return true
		}
	}

	return false
}
