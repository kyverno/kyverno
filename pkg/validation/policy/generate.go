package policy

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// checkClusterResourceInMatchAndExclude returns false if namespaced ClusterPolicy contains cluster wide resources in
// Match and Exclude block
func checkClusterResourceInMatchAndExclude(rule kyvernov1.Rule, clusterResources sets.Set[string], policyNamespace string, mock bool, res []*metav1.APIResourceList) error {
	if !mock {
		// Check for generate policy
		// - if resource to be generated is namespaced resource then the namespace field
		// should be mentioned
		// - if resource to be generated is non namespaced resource then the namespace field
		// should not be mentioned
		if rule.HasGenerate() {
			generateResourceKind := rule.Generation.Kind
			for _, resList := range res {
				for _, r := range resList.APIResources {
					if r.Kind == generateResourceKind {
						if r.Namespaced {
							if rule.Generation.Namespace == "" {
								return fmt.Errorf("path: spec.rules[%v]: please mention the namespace to generate a namespaced resource", rule.Name)
							}
							if rule.Generation.Namespace != policyNamespace {
								return fmt.Errorf("path: spec.rules[%v]: a namespaced policy cannot generate resources in other namespaces, expected: %v, received: %v", rule.Name, policyNamespace, rule.Generation.Namespace)
							}
							if rule.Generation.Clone.Name != "" {
								if rule.Generation.Clone.Namespace != policyNamespace {
									return fmt.Errorf("path: spec.rules[%v]: a namespaced policy cannot clone resources to or from other namespaces, expected: %v, received: %v", rule.Name, policyNamespace, rule.Generation.Clone.Namespace)
								}
							}
						} else {
							if rule.Generation.Namespace != "" {
								return fmt.Errorf("path: spec.rules[%v]: do not mention the namespace to generate a non namespaced resource", rule.Name)
							}
							if policyNamespace != "" {
								return fmt.Errorf("path: spec.rules[%v]: a namespaced policy cannot generate cluster-wide resources", rule.Name)
							}
						}
					} else if len(rule.Generation.CloneList.Kinds) != 0 {
						for _, kind := range rule.Generation.CloneList.Kinds {
							_, splitkind := kubeutils.GetKindFromGVK(kind)
							if r.Kind == splitkind {
								if r.Namespaced {
									if rule.Generation.CloneList.Namespace != policyNamespace {
										return fmt.Errorf("path: spec.rules[%v]: a namespaced policy cannot clone resource in other namespace, expected: %v, received: %v", rule.Name, policyNamespace, rule.Generation.Namespace)
									}
								} else {
									if policyNamespace != "" {
										return fmt.Errorf("path: spec.rules[%v]: a namespaced policy cannot generate cluster-wide resources", rule.Name)
									}
								}
							}
						}
					}
				}
			}
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
