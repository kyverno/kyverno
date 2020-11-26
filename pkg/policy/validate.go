package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/minio/minio/pkg/wildcard"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// Validate does some initial check to verify some conditions
// - One operation per rule
// - ResourceDescription mandatory checks
func Validate(policyRaw []byte, client *dclient.Client, mock bool, openAPIController *openapi.Controller) error {
	// check for invalid fields
	err := checkInvalidFields(policyRaw)
	if err != nil {
		return err
	}

	var p kyverno.ClusterPolicy
	err = json.Unmarshal(policyRaw, &p)
	if err != nil {
		return fmt.Errorf("failed to unmarshal policy: %v", err)
	}

	if len(common.PolicyHasVariables(p)) > 0 && common.PolicyHasNonAllowedVariables(p) {
		return fmt.Errorf("policy contains invalid variables")
	}

	if path, err := validateUniqueRuleName(p); err != nil {
		return fmt.Errorf("path: spec.%s: %v", path, err)
	}
	if p.Spec.Background == nil || *p.Spec.Background == true {
		if err := ContainsVariablesOtherThanObject(p); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule. %s ", err)
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

		if err := validateRuleContext(rule); err != nil {
			return fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		// validate Cluster Resources in namespaced policy
		// For namespaced policy, ClusterResource type field and values are not allowed in match and exclude
		if !mock && p.ObjectMeta.Namespace != "" {
			var Empty struct{}
			clusterResourcesMap := make(map[string]*struct{})
			// Get all the cluster type kind supported by cluster

			res, err := client.GetDiscoveryCache().ServerPreferredResources()
			if err != nil {
				return err
			}
			for _, resList := range res {
				for _, r := range resList.APIResources {
					if !r.Namespaced {
						if _, ok := clusterResourcesMap[r.Kind]; !ok {
							clusterResourcesMap[r.Kind] = &Empty
						}
					}
				}
			}

			clusterResources := make([]string, 0, len(clusterResourcesMap))
			for k := range clusterResourcesMap {
				clusterResources = append(clusterResources, k)
			}
			return checkClusterResourceInMatchAndExclude(rule, clusterResources)
		}

		if doMatchAndExcludeConflict(rule) {
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
		if !isLabelAndAnnotationsString(rule) {
			return fmt.Errorf("labels and annotations supports only string values, \"use double quotes around the non string values\"")
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

// checkInvalidFields - checks invalid fields in webhook policy request
// policy supports 5 json fields in types.go i.e. "apiVersion", "kind", "metadata", "spec", "status"
// If the webhook request policy contains new fields then block creation of policy
func checkInvalidFields(policyRaw []byte) error {
	// hardcoded supported fields by policy
	var allowedKeys = []string{"apiVersion", "kind", "metadata", "spec", "status"}
	var data interface{}
	err := json.Unmarshal(policyRaw, &data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal policy admission request err %v", err)
	}
	mapData := data.(map[string]interface{})
	// validate any new fields in the admission request against the supported fields and block the request with any new fields
	for requestField := range mapData {
		ok := false
		for _, allowedField := range allowedKeys {
			if requestField == allowedField {
				ok = true
				break
			}
		}

		if !ok {
			return fmt.Errorf("unknown field \"%s\" in policy", requestField)
		}
	}
	return nil
}

// doMatchAndExcludeConflict checks if the resultant
// of match and exclude block is not an empty set
func doMatchAndExcludeConflict(rule kyverno.Rule) bool {

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

	if (rule.MatchResources.ResourceDescription.Selector == nil && rule.ExcludeResources.ResourceDescription.Selector != nil) ||
		(rule.MatchResources.ResourceDescription.Selector != nil && rule.ExcludeResources.ResourceDescription.Selector == nil) ||
		(rule.MatchResources.ResourceDescription.Selector == nil && rule.ExcludeResources.ResourceDescription.Selector == nil) {
		return false
	}

	if rule.MatchResources.Annotations != nil && rule.ExcludeResources.Annotations != nil {
		if !(reflect.DeepEqual(rule.MatchResources.Annotations, rule.ExcludeResources.Annotations)) {
			return false
		}
	}

	if (rule.MatchResources.Annotations == nil && rule.ExcludeResources.Annotations != nil) ||
		(rule.MatchResources.Annotations != nil && rule.ExcludeResources.Annotations == nil) ||
		(rule.MatchResources.Annotations == nil && rule.ExcludeResources.Annotations == nil) {
		return false
	}

	return true
}

// isLabelAndAnnotationsString :- Validate if labels and annotations contains only string values
func isLabelAndAnnotationsString(rule kyverno.Rule) bool {
	// checkMetadata - Verify if the labels and annotations contains string value inside metadata
	checkMetadata := func(patternMap map[string]interface{}) bool {
		for k := range patternMap {
			if k == "metadata" {
				metaKey, ok := patternMap[k].(map[string]interface{})
				if ok {
					// range over metadata
					for mk := range metaKey {
						if mk == "labels" {
							labelKey, ok := metaKey[mk].(map[string]interface{})
							if ok {
								// range over labels
								for _, val := range labelKey {
									if reflect.TypeOf(val).String() != "string" {
										return false
									}
								}
							}
						} else if mk == "annotations" {
							annotationKey, ok := metaKey[mk].(map[string]interface{})
							if ok {
								// range over annotations
								for _, val := range annotationKey {
									if reflect.TypeOf(val).String() != "string" {
										return false
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

	patternMap, ok := rule.Validation.Pattern.(map[string]interface{})
	if ok {
		return checkMetadata(patternMap)
	} else if rule.Validation.AnyPattern != nil {
		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			log.Log.Error(err, "failed to deserialze anyPattern, expect type array")
			return false
		}

		for _, pattern := range anyPatterns {
			patternMap, ok := pattern.(map[string]interface{})
			if ok {
				ret := checkMetadata(patternMap)
				if ret == false {
					return ret
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

	anyPatterns, err := rule.Validation.DeserializeAnyPattern()
	if err != nil {
		log.Log.Error(err, "failed to deserialze anyPattern, expect type array")
		return false
	}

	for _, pattern := range anyPatterns {
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

func validateRuleContext(rule kyverno.Rule) error {
	if rule.Context == nil || len(rule.Context) == 0 {
		return nil
	}

	for _, entry := range rule.Context {
		if entry.Name == "" {
			return fmt.Errorf("a name is required for context entries")
		}

		if entry.ConfigMap != nil {
			if entry.ConfigMap.Name == "" {
				return fmt.Errorf("a name is required for configMap context entry")
			}

			if entry.ConfigMap.Namespace == "" {
				return fmt.Errorf("a namespace is required for configMap context entry")
			}
		}
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

// checkClusterResourceInMatchAndExclude returns false if namespaced ClusterPolicy contains cluster wide resources in
// Match and Exclude block
func checkClusterResourceInMatchAndExclude(rule kyverno.Rule, clusterResources []string) error {
	// Contains Namespaces in Match->ResourceDescription
	if len(rule.MatchResources.ResourceDescription.Namespaces) > 0 {
		return fmt.Errorf("namespaced cluster policy : field namespaces not allowed in match.resources")
	}
	// Contains Namespaces in Exclude->ResourceDescription
	if len(rule.ExcludeResources.ResourceDescription.Namespaces) > 0 {
		return fmt.Errorf("namespaced cluster policy : field namespaces not allowed in exclude.resources")
	}
	// Contains "Cluster Wide Resources" in Match->ResourceDescription->Kinds
	for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
		for _, k := range clusterResources {
			if kind == k {
				return fmt.Errorf("namespaced policy : cluster type value '%s' not allowed in match.resources.kinds", kind)
			}
		}
	}
	// Contains "Cluster Wide Resources" in Exclude->ResourceDescription->Kinds
	for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
		for _, k := range clusterResources {
			if kind == k {
				return fmt.Errorf("namespaced policy : cluster type value '%s' not allowed in exclude.resources.kinds", kind)
			}
		}

	}
	return nil
}
