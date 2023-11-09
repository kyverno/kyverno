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

func immutableGenerateFields(new, old kyvernov1.PolicyInterface) error {
	if new == nil || old == nil {
		return nil
	}

	if !new.GetSpec().HasGenerate() {
		return nil
	}

	oldRuleHashes, err := buildHashes(old.GetSpec().Rules)
	if err != nil {
		return err
	}
	newRuleHashes, err := buildHashes(new.GetSpec().Rules)
	if err != nil {
		return err
	}

	switch len(old.GetSpec().Rules) <= len(new.GetSpec().Rules) {
	case true:
		if newRuleHashes.IsSuperset(oldRuleHashes) {
			return nil
		} else {
			return errors.New("change of immutable fields for a generate rule is disallowed")
		}
	case false:
		if oldRuleHashes.IsSuperset(newRuleHashes) {
			return nil
		} else {
			return errors.New("rule deletion - change of immutable fields for a generate rule is disallowed")
		}
	}
	return nil
}

func resetMutableFields(rule kyvernov1.Rule) *kyvernov1.Rule {
	new := new(kyvernov1.Rule)
	rule.DeepCopyInto(new)
	new.Generation.Synchronize = true
	new.Generation.SetData(nil)
	return new
}

func buildHashes(rules []kyvernov1.Rule) (sets.Set[string], error) {
	ruleHashes := sets.New[string]()
	for _, rule := range rules {
		r := resetMutableFields(rule)
		data, err := json.Marshal(r)
		if err != nil {
			return ruleHashes, fmt.Errorf("failed to create hash from the generate rule %v", err)
		}
		hash := md5.Sum(data) //nolint:gosec
		ruleHashes.Insert(hex.EncodeToString(hash[:]))
	}
	return ruleHashes, nil
}
