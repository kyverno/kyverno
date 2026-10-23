package policy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

	changed, err := changedImmutableRules(oldRuleHashes, newRuleHashes)
	if err != nil {
		return "", err
	}
	if len(changed) > 0 {
		return "", fmt.Errorf("changes of immutable fields of a rule spec in a generate rule is disallowed: %s; add a new rule instead of changing the existing one, or delete and recreate the policy", strings.Join(changed, ", "))
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

// buildHashes returns the hashes of the immutable part of every generate rule, keyed by hash and
// mapped to the rule (after resetMutableFields) they were computed from, and the set of hashes of
// the generate patterns.
func buildHashes(rules []kyvernov1.Rule, oldHasSynchronizingRule bool) (ruleHashes map[string]*kyvernov1.Rule, generationHashes sets.Set[string], _ error) {
	ruleHashes, generationHashes = map[string]*kyvernov1.Rule{}, sets.New[string]()

	for _, rule := range rules {
		if !rule.HasGenerate() {
			continue
		}
		r, generation := resetMutableFields(rule, oldHasSynchronizingRule)
		data, err := json.Marshal(generation)
		if err != nil {
			return ruleHashes, generationHashes, fmt.Errorf("failed to create hash from the generate rule %v", err)
		}
		hash := md5.Sum(data)
		generationHashes.Insert(hex.EncodeToString(hash[:]))

		data, err = json.Marshal(r)
		if err != nil {
			return ruleHashes, generationHashes, fmt.Errorf("failed to create hash from the generate rule %v", err)
		}
		hash = md5.Sum(data)
		ruleHashes[hex.EncodeToString(hash[:])] = r
	}
	return ruleHashes, generationHashes, nil
}

// changedImmutableRules describes every old generate rule whose immutable fields have no identical
// counterpart in the new policy: the rule name and, when a rule with the same name still exists, the
// top-level fields that differ between the two rules after resetMutableFields.
func changedImmutableRules(oldRuleHashes, newRuleHashes map[string]*kyvernov1.Rule) ([]string, error) {
	newRulesByName := make(map[string]*kyvernov1.Rule, len(newRuleHashes))
	for _, rule := range newRuleHashes {
		newRulesByName[rule.Name] = rule
	}

	var changed []string
	for hash, oldRule := range oldRuleHashes {
		if _, ok := newRuleHashes[hash]; ok {
			continue
		}
		newRule, ok := newRulesByName[oldRule.Name]
		if !ok {
			changed = append(changed, fmt.Sprintf("rule %q was removed or renamed", oldRule.Name))
			continue
		}
		fields, err := changedRuleFields(oldRule, newRule)
		if err != nil {
			return nil, err
		}
		changed = append(changed, fmt.Sprintf("rule %q changed fields [%s]", oldRule.Name, strings.Join(fields, ", ")))
	}
	sort.Strings(changed)
	return changed, nil
}

func changedRuleFields(oldRule, newRule *kyvernov1.Rule) ([]string, error) {
	oldFields, err := ruleFields(oldRule)
	if err != nil {
		return nil, err
	}
	newFields, err := ruleFields(newRule)
	if err != nil {
		return nil, err
	}
	changed := sets.New[string]()
	for name, value := range oldFields {
		if !bytes.Equal(value, newFields[name]) {
			changed.Insert(name)
		}
	}
	for name := range newFields {
		if _, ok := oldFields[name]; !ok {
			changed.Insert(name)
		}
	}
	return sets.List(changed), nil
}

func ruleFields(rule *kyvernov1.Rule) (map[string]json.RawMessage, error) {
	data, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the generate rule %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the generate rule %v", err)
	}
	return fields, nil
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
