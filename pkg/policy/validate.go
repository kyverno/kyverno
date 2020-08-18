package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/minio/minio/pkg/wildcard"

	"github.com/nirmata/kyverno/pkg/openapi"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate does some initial check to verify some conditions
// - One operation per rule
// - ResourceDescription mandatory checks
func Validate(policyRaw []byte, client *dclient.Client, mock bool, openAPIController *openapi.Controller) error {
	var p kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &p)
	if err != nil {
		return fmt.Errorf("failed to unmarshal policy admission request err %v", err)
	}

	if path, err := validateUniqueRuleName(p); err != nil {
		return fmt.Errorf("path: spec.%s: %v", path, err)
	}
	if p.Spec.Background == nil || (p.Spec.Background != nil && *p.Spec.Background) {
		if err := ContainsVariablesOtherThanObject(p); err != nil {
			return fmt.Errorf("only variables referring request.object are allowed in background mode. Set spec.background=false to disable background mode for this policy rule. %s ", err)
		}
	}

	for i, rule := range p.Spec.Rules {
		// validate resource description
		if path, err := validateResources(rule); err != nil {
			return fmt.Errorf("path: spec.rules[%d].%s: %v", i, path, err)
		}
		// validate rule types
		// only one type of rule is allowed per rule
		if err := validateRuleType(rule); err != nil {
			// as there are more than 1 operation in rule, not need to evaluate it further
			return fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		if doesMatchAndExcludeConflict(rule) {
			return fmt.Errorf("path: spec.rules[%v]: rule is matching an empty set", rule.Name)
		}

		// validate rule actions
		// - Mutate
		// - Validate
		// - Generate
		if err := validateActions(i, rule, client, mock); err != nil {
			return err
		}

		// If a rules match block does not match any kind,
		// we should only allow such rules to have metadata in its overlay
		if len(rule.MatchResources.Kinds) == 0 {
			if !ruleOnlyDealsWithResourceMetaData(rule) {
				return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
					" the rule does not match an kind")
			}
		}

		// Validate string values in labels
		if !isLabelString(rule){
			return fmt.Errorf("labels supports only string values, \"use double quotes around the non string values\"")
		}
	}

	if !mock {
		if err := openAPIController.ValidatePolicyFields(policyRaw); err != nil {
			return err
		}
	} else {
		if err := openAPIController.ValidatePolicyMutation(p); err != nil {
			return err
		}
	}

	return nil
}

// doesMatchAndExcludeConflict checks if the resultant
// of match and exclude block is not an empty set
func doesMatchAndExcludeConflict(rule kyverno.Rule) bool {

	if reflect.DeepEqual(rule.ExcludeResources, kyverno.ExcludeResources{}) {
		return false
	}

	excludeRoles := make(map[string]bool)
	for _, role := range rule.ExcludeResources.UserInfo.Roles {
		excludeRoles[role] = true
	}

	excludeClusterRoles := make(map[string]bool)
	for _, clusterRoles := range rule.ExcludeResources.UserInfo.ClusterRoles {
		excludeClusterRoles[clusterRoles] = true
	}

	excludeSubjects := make(map[string]bool)
	for _, subject := range rule.ExcludeResources.UserInfo.Subjects {
		subjectRaw, _ := json.Marshal(subject)
		excludeSubjects[string(subjectRaw)] = true
	}

	excludeKinds := make(map[string]bool)
	for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
		excludeKinds[kind] = true
	}

	excludeNamespaces := make(map[string]bool)
	for _, namespace := range rule.ExcludeResources.ResourceDescription.Namespaces {
		excludeNamespaces[namespace] = true
	}

	excludeMatchExpressions := make(map[string]bool)
	if rule.ExcludeResources.ResourceDescription.Selector != nil {
		for _, matchExpression := range rule.ExcludeResources.ResourceDescription.Selector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeMatchExpressions[string(matchExpressionRaw)] = true
		}
	}

	if len(excludeRoles) > 0 {
		if len(rule.MatchResources.UserInfo.Roles) == 0 {
			return false
		}

		for _, role := range rule.MatchResources.UserInfo.Roles {
			if !excludeRoles[role] {
				return false
			}
		}
	}

	if len(excludeClusterRoles) > 0 {
		if len(rule.MatchResources.UserInfo.ClusterRoles) == 0 {
			return false
		}

		for _, clusterRole := range rule.MatchResources.UserInfo.ClusterRoles {
			if !excludeClusterRoles[clusterRole] {
				return false
			}
		}
	}

	if len(excludeSubjects) > 0 {
		if len(rule.MatchResources.UserInfo.Subjects) == 0 {
			return false
		}

		for _, subject := range rule.MatchResources.UserInfo.Subjects {
			subjectRaw, _ := json.Marshal(subject)
			if !excludeSubjects[string(subjectRaw)] {
				return false
			}
		}
	}

	if rule.ExcludeResources.ResourceDescription.Name != "" {
		if !wildcard.Match(rule.ExcludeResources.ResourceDescription.Name, rule.MatchResources.ResourceDescription.Name) {
			return false
		}
	}

	if len(excludeNamespaces) > 0 {
		if len(rule.MatchResources.ResourceDescription.Namespaces) == 0 {
			return false
		}

		for _, namespace := range rule.MatchResources.ResourceDescription.Namespaces {
			if !excludeNamespaces[namespace] {
				return false
			}
		}
	}

	if len(excludeKinds) > 0 {
		if len(rule.MatchResources.ResourceDescription.Kinds) == 0 {
			return false
		}

		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			if !excludeKinds[kind] {
				return false
			}
		}
	}

	if rule.MatchResources.ResourceDescription.Selector != nil && rule.ExcludeResources.ResourceDescription.Selector != nil {
		if len(excludeMatchExpressions) > 0 {
			if len(rule.MatchResources.ResourceDescription.Selector.MatchExpressions) == 0 {
				return false
			}

			for _, matchExpression := range rule.MatchResources.ResourceDescription.Selector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeMatchExpressions[string(matchExpressionRaw)] {
					return false
				}
			}
		}

		if len(rule.ExcludeResources.ResourceDescription.Selector.MatchLabels) > 0 {
			if len(rule.MatchResources.ResourceDescription.Selector.MatchLabels) == 0 {
				return false
			}

			for label, value := range rule.MatchResources.ResourceDescription.Selector.MatchLabels {
				if rule.ExcludeResources.ResourceDescription.Selector.MatchLabels[label] != value {
					return false
				}
			}
		}
	}

	return true
}
// isLabelString :- Validate if labels contains only string values
func isLabelString(rule kyverno.Rule) bool {
	patternMap, ok := rule.Validation.Pattern.(map[string]interface{})
	if ok {
		for k := range patternMap {
			if k == "metadata" {
				metaKey, ok := patternMap[k].(map[string]interface{})
				if ok {
					// range over metadata
					for mk := range metaKey {
						if mk == "labels"{
							labelKey, ok := metaKey[mk].(map[string]interface{})
							if ok {
								// range over labels
								for _, val := range labelKey {
									if reflect.TypeOf(val).String() != "string"{
										return false
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return true
}

func ruleOnlyDealsWithResourceMetaData(rule kyverno.Rule) bool {
	overlayMap, _ := rule.Mutation.Overlay.(map[string]interface{})
	for k := range overlayMap {
		if k != "metadata" {
			return false
		}
	}

	for _, patch := range rule.Mutation.Patches {
		if !strings.HasPrefix(patch.Path, "/metadata") {
			return false
		}
	}

	patternMap, _ := rule.Validation.Pattern.(map[string]interface{})
	for k := range patternMap {
		if k != "metadata" {
			return false
		}
	}

	for _, pattern := range rule.Validation.AnyPattern {
		patternMap, _ := pattern.(map[string]interface{})
		for k := range patternMap {
			if k != "metadata" {
				return false
			}
		}
	}

	return true
}

func validateResources(rule kyverno.Rule) (string, error) {
	// validate userInfo in match and exclude
	if path, err := validateUserInfo(rule); err != nil {
		return fmt.Sprintf("resources.%s", path), err
	}

	// matched resources
	if path, err := validateMatchedResourceDescription(rule.MatchResources.ResourceDescription); err != nil {
		return fmt.Sprintf("resources.%s", path), err
	}
	// exclude resources
	if path, err := validateExcludeResourceDescription(rule.ExcludeResources.ResourceDescription); err != nil {
		return fmt.Sprintf("resources.%s", path), err
	}
	return "", nil
}

// ValidateUniqueRuleName checks if the rule names are unique across a policy
func validateUniqueRuleName(p kyverno.ClusterPolicy) (string, error) {
	var ruleNames []string

	for i, rule := range p.Spec.Rules {
		if containString(ruleNames, rule.Name) {
			return fmt.Sprintf("rule[%d]", i), fmt.Errorf(`duplicate rule name: '%s'`, rule.Name)
		}
		ruleNames = append(ruleNames, rule.Name)
	}
	return "", nil
}

// validateRuleType checks only one type of rule is defined per rule
func validateRuleType(r kyverno.Rule) error {
	ruleTypes := []bool{r.HasMutate(), r.HasValidate(), r.HasGenerate()}

	operationCount := func() int {
		count := 0
		for _, v := range ruleTypes {
			if v {
				count++
			}
		}
		return count
	}()

	if operationCount == 0 {
		return fmt.Errorf("no operation defined in the rule '%s'.(supported operations: mutation,validation,generation)", r.Name)
	} else if operationCount != 1 {
		return fmt.Errorf("multiple operations defined in the rule '%s', only one type of operation is allowed per rule", r.Name)
	}
	return nil
}

// validateResourceDescription checks if all necesarry fields are present and have values. Also checks a Selector.
// field type is checked through openapi
// Returns error if
// - kinds is empty array in matched resource block, i.e. kinds: []
// - selector is invalid
func validateMatchedResourceDescription(rd kyverno.ResourceDescription) (string, error) {
	if reflect.DeepEqual(rd, kyverno.ResourceDescription{}) {
		return "", fmt.Errorf("match resources not specified")
	}

	if err := validateResourceDescription(rd); err != nil {
		return "match", err
	}

	return "", nil
}

func validateUserInfo(rule kyverno.Rule) (string, error) {
	if err := validateRoles(rule.MatchResources.Roles); err != nil {
		return "match.roles", err
	}

	if err := validateSubjects(rule.MatchResources.Subjects); err != nil {
		return "match.subjects", err
	}

	if err := validateRoles(rule.ExcludeResources.Roles); err != nil {
		return "exclude.roles", err
	}

	if err := validateSubjects(rule.ExcludeResources.Subjects); err != nil {
		return "exclude.subjects", err
	}

	return "", nil
}

// a role must in format namespace:name
func validateRoles(roles []string) error {
	if len(roles) == 0 {
		return nil
	}

	for _, r := range roles {
		role := strings.Split(r, ":")
		if len(role) != 2 {
			return fmt.Errorf("invalid role %s, expect namespace:name", r)
		}
	}
	return nil
}

// a namespace should be set in kind ServiceAccount of a subject
func validateSubjects(subjects []rbacv1.Subject) error {
	if len(subjects) == 0 {
		return nil
	}

	for _, subject := range subjects {
		if subject.Kind == "ServiceAccount" {
			if subject.Namespace == "" {
				return fmt.Errorf("service account %s in subject expects a namespace", subject.Name)
			}
		}
	}
	return nil
}

func validateExcludeResourceDescription(rd kyverno.ResourceDescription) (string, error) {
	if reflect.DeepEqual(rd, kyverno.ResourceDescription{}) {
		// exclude is not mandatory
		return "", nil
	}
	if err := validateResourceDescription(rd); err != nil {
		return "exclude", err
	}
	return "", nil
}

// validateResourceDescription returns error if selector is invalid
// field type is checked through openapi
func validateResourceDescription(rd kyverno.ResourceDescription) error {
	if rd.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(rd.Selector)
		if err != nil {
			return err
		}
		requirements, _ := selector.Requirements()
		if len(requirements) == 0 {
			return errors.New("the requirements are not specified in selector")
		}
	}
	return nil
}
