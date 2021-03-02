package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jmespath/go-jmespath"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/kyverno/common"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/minio/minio/pkg/wildcard"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// Validate does some initial check to verify some conditions
// - One operation per rule
// - ResourceDescription mandatory checks
func Validate(policy *kyverno.ClusterPolicy, client *dclient.Client, mock bool, openAPIController *openapi.Controller) error {
	p := *policy
	if len(common.PolicyHasVariables(p)) > 0 && common.PolicyHasNonAllowedVariables(p) {
		return fmt.Errorf("policy contains invalid variables")
	}

	// policy name is stored in the label of the report change request
	if len(p.Name) > 63 {
		return fmt.Errorf("invalid policy name %s: must be no more than 63 characters", p.Name)
	}

	if path, err := validateUniqueRuleName(p); err != nil {
		return fmt.Errorf("path: spec.%s: %v", path, err)
	}
	if p.Spec.Background == nil || *p.Spec.Background == true {
		if err := ContainsVariablesOtherThanObject(p); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule: %s ", err)
		}
	}

	for i, rule := range p.Spec.Rules {
		if jsonPatchOnPod(rule) {
			log.Log.V(1).Info("warning: pods managed by workload controllers cannot be mutated using policies. Use the auto-gen feature or write policies that match pod controllers.")
		}
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

			res, err := client.DiscoveryClient.DiscoveryCache().ServerPreferredResources()
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

		// add label to source mentioned in policy
		if !mock && rule.Generation.Clone.Name != "" {
			obj, err := client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
			if err != nil {
				log.Log.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
				continue
			}

			updateSource := true
			label := obj.GetLabels()

			if len(label) == 0 {
				label = make(map[string]string)
				label["generate.kyverno.io/clone-policy-name"] = p.GetName()
			} else {
				if label["generate.kyverno.io/clone-policy-name"] != "" {
					policyNames := label["generate.kyverno.io/clone-policy-name"]
					if !strings.Contains(policyNames, p.GetName()) {
						policyNames = policyNames + "," + p.GetName()
						label["generate.kyverno.io/clone-policy-name"] = policyNames
					} else {
						updateSource = false
					}
				} else {
					label["generate.kyverno.io/clone-policy-name"] = p.GetName()
				}
			}

			if updateSource {
				log.Log.V(4).Info("updating existing clone source")
				obj.SetLabels(label)
				_, err = client.UpdateResource(obj.GetAPIVersion(), rule.Generation.Kind, rule.Generation.Clone.Namespace, obj, false)
				if err != nil {
					log.Log.Error(err, "failed to update source  name:%v namespace:%v kind:%v", obj.GetName(), obj.GetNamespace(), obj.GetKind())
					continue
				}
				log.Log.V(4).Info("updated source  name:%v namespace:%v kind:%v", obj.GetName(), obj.GetNamespace(), obj.GetKind())
			}
		}
	}

	if !mock {
		if err := openAPIController.ValidatePolicyFields(p); err != nil {
			return err
		}
	} else {
		if err := openAPIController.ValidatePolicyMutation(p); err != nil {
			return err
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

	excludeSelectorMatchExpressions := make(map[string]bool)
	if rule.ExcludeResources.ResourceDescription.Selector != nil {
		for _, matchExpression := range rule.ExcludeResources.ResourceDescription.Selector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeSelectorMatchExpressions[string(matchExpressionRaw)] = true
		}
	}

	excludeNamespaceSelectorMatchExpressions := make(map[string]bool)
	if rule.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
		for _, matchExpression := range rule.ExcludeResources.ResourceDescription.NamespaceSelector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeNamespaceSelectorMatchExpressions[string(matchExpressionRaw)] = true
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
		if len(excludeSelectorMatchExpressions) > 0 {
			if len(rule.MatchResources.ResourceDescription.Selector.MatchExpressions) == 0 {
				return false
			}

			for _, matchExpression := range rule.MatchResources.ResourceDescription.Selector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeSelectorMatchExpressions[string(matchExpressionRaw)] {
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

	if rule.MatchResources.ResourceDescription.NamespaceSelector != nil && rule.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
		if len(excludeNamespaceSelectorMatchExpressions) > 0 {
			if len(rule.MatchResources.ResourceDescription.NamespaceSelector.MatchExpressions) == 0 {
				return false
			}

			for _, matchExpression := range rule.MatchResources.ResourceDescription.NamespaceSelector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeNamespaceSelectorMatchExpressions[string(matchExpressionRaw)] {
					return false
				}
			}
		}

		if len(rule.ExcludeResources.ResourceDescription.NamespaceSelector.MatchLabels) > 0 {
			if len(rule.MatchResources.ResourceDescription.NamespaceSelector.MatchLabels) == 0 {
				return false
			}

			for label, value := range rule.MatchResources.ResourceDescription.NamespaceSelector.MatchLabels {
				if rule.ExcludeResources.ResourceDescription.NamespaceSelector.MatchLabels[label] != value {
					return false
				}
			}
		}
	}

	if (rule.MatchResources.ResourceDescription.Selector == nil && rule.ExcludeResources.ResourceDescription.Selector != nil) ||
		(rule.MatchResources.ResourceDescription.Selector != nil && rule.ExcludeResources.ResourceDescription.Selector == nil) {
		return false
	}

	if (rule.MatchResources.ResourceDescription.NamespaceSelector == nil && rule.ExcludeResources.ResourceDescription.NamespaceSelector != nil) ||
		(rule.MatchResources.ResourceDescription.NamespaceSelector != nil && rule.ExcludeResources.ResourceDescription.NamespaceSelector == nil) {
		return false
	}

	if rule.MatchResources.Annotations != nil && rule.ExcludeResources.Annotations != nil {
		if !(reflect.DeepEqual(rule.MatchResources.Annotations, rule.ExcludeResources.Annotations)) {
			return false
		}
	}

	if (rule.MatchResources.Annotations == nil && rule.ExcludeResources.Annotations != nil) ||
		(rule.MatchResources.Annotations != nil && rule.ExcludeResources.Annotations == nil) {
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
			log.Log.Error(err, "failed to deserialize anyPattern, expect type array")
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
		log.Log.Error(err, "failed to deserialize anyPattern, expect type array")
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
		return fmt.Sprintf("match.resources.%s", path), err
	}
	// exclude resources
	if path, err := validateExcludeResourceDescription(rule.ExcludeResources.ResourceDescription); err != nil {
		return fmt.Sprintf("exclude.resources.%s", path), err
	}

	//validating the values present under validate.preconditions, if they exist
	if rule.AnyAllConditions != nil {
		if path, err := validateConditions(rule.AnyAllConditions, "preconditions"); err != nil {
			return fmt.Sprintf("validate.%s", path), err
		}
	}
	//validating the values present under validate.conditions, if they exist
	if rule.Validation.Deny != nil && rule.Validation.Deny.AnyAllConditions != nil {
		if path, err := validateConditions(rule.Validation.Deny.AnyAllConditions, "conditions"); err != nil {
			return fmt.Sprintf("validate.deny.%s", path), err
		}
	}
	return "", nil
}

// validateConditions validates all the 'conditions' or 'preconditions' of a rule depending on the corresponding 'condition.key'.
// As of now, it is validating the 'value' field whether it contains the only allowed set of values or not when 'condition.key' is {{request.operation}}
// this is backwards compatible i.e. conditions can be provided in the old manner as well i.e. without 'any' or 'all'
func validateConditions(conditions apiextensions.JSON, schemaKey string) (string, error) {
	// Conditions can only exist under some specific keys of the policy schema
	allowedSchemaKeys := map[string]bool{
		"preconditions": true,
		"conditions":    true,
	}
	if !allowedSchemaKeys[schemaKey] {
		return fmt.Sprintf(schemaKey), fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}

	// conditions are currently in the form of []interface{}
	kyvernoConditions, err := utils.ApiextensionsJsonToKyvernoConditions(conditions)
	if err != nil {
		return fmt.Sprintf("%s", schemaKey), err
	}
	switch typedConditions := kyvernoConditions.(type) {
	case kyverno.AnyAllConditions:
		// validating the conditions under 'any', if there are any
		if !reflect.DeepEqual(typedConditions, kyverno.AnyAllConditions{}) && typedConditions.AnyConditions != nil {
			for i, condition := range typedConditions.AnyConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.any[%d].%s", schemaKey, i, path), err
				}
			}
		}
		// validating the conditions under 'all', if there are any
		if !reflect.DeepEqual(typedConditions, kyverno.AnyAllConditions{}) && typedConditions.AllConditions != nil {
			for i, condition := range typedConditions.AllConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.all[%d].%s", schemaKey, i, path), err
				}
			}
		}

	case []kyverno.Condition: // backwards compatibility
		for i, condition := range typedConditions {
			if path, err := validateConditionValues(condition); err != nil {
				return fmt.Sprintf("%s[%d].%s", schemaKey, i, path), err
			}
		}
	}
	return "", nil
}

// validateConditionValues validates whether all the values under the 'value' field of a 'conditions' field
// are apt with respect to the provided 'condition.key'
func validateConditionValues(c kyverno.Condition) (string, error) {
	switch strings.ReplaceAll(c.Key.(string), " ", "") {
	case "{{request.operation}}":
		return validateConditionValuesKeyRequestOperation(c)
	default:
		return "", nil
	}
}

// validateConditionValuesKeyRequestOperation validates whether all the values under the 'value' field of a 'conditions' field
// are one of ["CREATE", "UPDATE", "DELETE", "CONNECT"] when 'condition.key' is {{request.operation}}
func validateConditionValuesKeyRequestOperation(c kyverno.Condition) (string, error) {
	valuesAllowed := map[string]bool{
		"CREATE":  true,
		"UPDATE":  true,
		"DELETE":  true,
		"CONNECT": true,
	}
	switch reflect.TypeOf(c.Value).Kind() {
	case reflect.String:
		valueStr := c.Value.(string)
		// allow templatized values like {{ config-map.data.sample-key }}
		// because they might be actually pointing to a rightful value in the provided config-map
		if len(valueStr) >= 4 && valueStr[:2] == "{{" && valueStr[len(valueStr)-2:] == "}}" {
			return "", nil
		}
		if !valuesAllowed[valueStr] {
			return fmt.Sprintf("value: %s", c.Value.(string)), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", c.Value.(string))
		}
	case reflect.Slice:
		values := reflect.ValueOf(c.Value)
		for i := 0; i < values.Len(); i++ {
			value := values.Index(i).Interface().(string)
			if !valuesAllowed[value] {
				return fmt.Sprintf("value[%d]", i), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", value)
			}
		}
	default:
		return fmt.Sprintf("value"), fmt.Errorf("'value' field found to be of the type %v. The provided value/values are expected to be either in the form of a string or list", reflect.TypeOf(c.Value).Kind())
	}
	return "", nil
}

// validateUniqueRuleName checks if the rule names are unique across a policy
func validateUniqueRuleName(p kyverno.ClusterPolicy) (string, error) {
	var ruleNames []string

	for i, rule := range p.Spec.Rules {
		if utils.ContainsString(ruleNames, rule.Name) {
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

		var err error
		if entry.ConfigMap != nil {
			err = validateConfigMap(entry)
		} else if entry.APICall != nil {
			err = validateAPICall(entry)
		} else {
			return fmt.Errorf("a configMap or apiCall is required for context entries")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func validateConfigMap(entry kyverno.ContextEntry) error {
	if entry.ConfigMap == nil {
		return fmt.Errorf("configMap is empty")
	}

	if entry.APICall != nil {
		return fmt.Errorf("both configMap and apiCall are not allowed in a context entry")
	}

	if entry.ConfigMap.Name == "" {
		return fmt.Errorf("a name is required for configMap context entry")
	}

	if entry.ConfigMap.Namespace == "" {
		return fmt.Errorf("a namespace is required for configMap context entry")
	}

	return nil
}

func validateAPICall(entry kyverno.ContextEntry) error {
	if entry.APICall == nil {
		return fmt.Errorf("apiCall is empty")
	}

	if entry.ConfigMap != nil {
		return fmt.Errorf("both configMap and apiCall are not allowed in a context entry")
	}

	if _, err := engine.NewAPIPath(entry.APICall.URLPath); err != nil {
		return err
	}

	if entry.APICall.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.APICall.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.APICall.JMESPath, err)
		}
	}

	return nil
}

// validateResourceDescription checks if all necessary fields are present and have values. Also checks a Selector.
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

// jsonPatchOnPod checks if a rule applies JSON patches to Pod
func jsonPatchOnPod(rule kyverno.Rule) bool {
	if !rule.HasMutate() {
		return false
	}

	if utils.ContainsString(rule.MatchResources.Kinds, "Pod") && rule.Mutation.PatchesJSON6902 != "" {
		return true
	}

	return false
}
