package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/context"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	comn "github.com/kyverno/kyverno/pkg/common"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/kyverno/common"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/minio/pkg/wildcard"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var allowedVariables = regexp.MustCompile(`request\.|serviceAccountName|serviceAccountNamespace|element\.|@|images\.|([a-z_0-9]+\()[^{}]`)

var allowedVariablesBackground = regexp.MustCompile(`request\.|element\.|@|images\.|([a-z_0-9]+\()[^{}]`)

// wildCardAllowedVariables represents regex for the allowed fields in wildcards
var wildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace)[^{}]*\}\}`)

// validateJSONPatchPathForForwardSlash checks for forward slash
func validateJSONPatchPathForForwardSlash(patch string) error {

	re, err := regexp.Compile("^/")
	if err != nil {
		return err
	}

	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		path, err := operation.Path()
		if err != nil {
			return err
		}

		val := re.MatchString(path)

		if !val {
			return fmt.Errorf("%s", path)
		}

	}
	return nil
}

// Validate checks the policy and rules declarations for required configurations
func Validate(policy *kyverno.ClusterPolicy, client *dclient.Client, mock bool, openAPIController *openapi.Controller) error {
	namespaced := false
	background := policy.Spec.Background == nil || *policy.Spec.Background

	clusterResources := make([]string, 0)
	err := ValidateVariables(policy, background)
	if err != nil {
		return err
	}

	// policy name is stored in the label of the report change request
	if len(policy.Name) > 63 {
		return fmt.Errorf("invalid policy name %s: must be no more than 63 characters", policy.Name)
	}

	if path, err := validateUniqueRuleName(*policy); err != nil {
		return fmt.Errorf("path: spec.%s: %v", path, err)
	}

	if policy.ObjectMeta.Namespace != "" {
		namespaced = true
	}

	var res []*metav1.APIResourceList

	if !mock && namespaced {
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

		for k := range clusterResourcesMap {
			clusterResources = append(clusterResources, k)
		}
	}

	for i, rule := range policy.Spec.Rules {
		//check for forward slash
		if err := validateJSONPatchPathForForwardSlash(rule.Mutation.PatchesJSON6902); err != nil {
			return fmt.Errorf("path must begin with a forward slash: spec.rules[%d]: %s", i, err)
		}

		if jsonPatchOnPod(rule) {
			log.Log.V(1).Info("pods managed by workload controllers cannot be mutated using policies, use the auto-gen feature or write policies that match pod controllers")
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

		err := validateElementInForEach(rule)
		if err != nil {
			return err
		}

		if err := validateRuleContext(rule); err != nil {
			return fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		// validate Cluster Resources in namespaced policy
		// For namespaced policy, ClusterResource type field and values are not allowed in match and exclude
		if namespaced {
			return checkClusterResourceInMatchAndExclude(rule, clusterResources, mock, res)
		}

		if doMatchAndExcludeConflict(rule) {
			return fmt.Errorf("path: spec.rules[%v]: rule is matching an empty set", rule.Name)
		}

		// validate rule actions
		// - Mutate
		// - Validate
		// - Generate
		if err := validateActions(i, &policy.Spec.Rules[i], client, mock); err != nil {
			return err
		}

		// If a rule's match block does not match any kind,
		// we should only allow it to have metadata in its overlay
		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				if len(rmr.Kinds) == 0 {
					return validateMatchKindHelper(rule)
				}
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				if len(rmr.Kinds) == 0 {
					return validateMatchKindHelper(rule)
				}
			}
		} else {
			if len(rule.MatchResources.Kinds) == 0 {
				return validateMatchKindHelper(rule)
			}
		}

		if utils.ContainsString(rule.MatchResources.Kinds, "*") && (policy.Spec.Background == nil || *policy.Spec.Background) {
			return fmt.Errorf("wildcard policy not allowed in background mode. Set spec.background=false to disable background mode for this policy rule ")
		}

		if (utils.ContainsString(rule.MatchResources.Kinds, "*") && len(rule.MatchResources.Kinds) > 1) || (utils.ContainsString(rule.ExcludeResources.Kinds, "*") && len(rule.ExcludeResources.Kinds) > 1) {
			return fmt.Errorf("wildard policy can not deal more than one kind")
		}

		if utils.ContainsString(rule.MatchResources.Kinds, "*") || utils.ContainsString(rule.ExcludeResources.Kinds, "*") {

			if rule.HasGenerate() || rule.HasVerifyImages() || rule.Validation.ForEachValidation != nil {
				return fmt.Errorf("wildcard policy does not support rule type")
			}

			if rule.HasValidate() {

				if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
					if !ruleOnlyDealsWithResourceMetaData(rule) {
						return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
							" the rule does not match any kind")
					}
				}

				if rule.Validation.Deny != nil {
					kyvernoConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.Validation.Deny.AnyAllConditions)
					switch typedConditions := kyvernoConditions.(type) {
					case []kyverno.Condition: // backwards compatibility
						for _, condition := range typedConditions {
							if !strings.Contains(condition.Key.(string), "request.object.metadata.") && (!wildCardAllowedVariables.MatchString(condition.Key.(string)) || strings.Contains(condition.Key.(string), "request.object.spec")) {
								return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
									" the rule does not match any kind")
							}
						}
					}
				}
			}

			if rule.HasMutate() {
				if !ruleOnlyDealsWithResourceMetaData(rule) {
					return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
						" the rule does not match any kind")
				}
			}

			if rule.HasVerifyImages() {
				for _, i := range rule.VerifyImages {
					if err := validateVerifyImagesRule(i); err != nil {
						return errors.Wrapf(err, "failed to validate policy %s rule %s", policy.Name, rule.Name)
					}
				}
			}
		}

		//Validate Kind with match resource kinds
		match := rule.MatchResources
		exclude := rule.ExcludeResources
		for _, value := range match.Any {
			err := validateKinds(value.ResourceDescription.Kinds, mock, client, *policy)
			if err != nil {
				return fmt.Errorf("the kind defined in the any match resource is invalid")
			}
		}
		for _, value := range match.All {
			err := validateKinds(value.ResourceDescription.Kinds, mock, client, *policy)
			if err != nil {
				return fmt.Errorf("the kind defined in the all match resource is invalid")
			}
		}
		for _, value := range exclude.Any {
			err := validateKinds(value.ResourceDescription.Kinds, mock, client, *policy)

			if err != nil {
				return fmt.Errorf("the kind defined in the any exclude resource is invalid")
			}
		}
		for _, value := range exclude.All {
			err := validateKinds(value.ResourceDescription.Kinds, mock, client, *policy)
			if err != nil {
				return fmt.Errorf("the kind defined in the all exclude resource is invalid")
			}
		}
		if !utils.ContainsString(rule.MatchResources.Kinds, "*") {
			err := validateKinds(rule.MatchResources.Kinds, mock, client, *policy)
			if err != nil {
				return errors.Wrapf(err, "match resource kind is invalid")
			}
			err = validateKinds(rule.ExcludeResources.Kinds, mock, client, *policy)
			if err != nil {
				return errors.Wrapf(err, "exclude resource kind is invalid")
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
				label["generate.kyverno.io/clone-policy-name"] = policy.GetName()
			} else {
				if label["generate.kyverno.io/clone-policy-name"] != "" {
					policyNames := label["generate.kyverno.io/clone-policy-name"]
					if !strings.Contains(policyNames, policy.GetName()) {
						policyNames = policyNames + "," + policy.GetName()
						label["generate.kyverno.io/clone-policy-name"] = policyNames
					} else {
						updateSource = false
					}
				} else {
					label["generate.kyverno.io/clone-policy-name"] = policy.GetName()
				}
			}

			if updateSource {
				log.Log.V(4).Info("updating existing clone source")
				obj.SetLabels(label)
				_, err = client.UpdateResource(obj.GetAPIVersion(), rule.Generation.Kind, rule.Generation.Clone.Namespace, obj, false)
				if err != nil {
					log.Log.Error(err, "failed to update source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
					continue
				}
				log.Log.V(4).Info("updated source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			}
		}
	}

	if !mock {
		if err := openAPIController.ValidatePolicyFields(*policy); err != nil {
			return err
		}
	} else {
		if err := openAPIController.ValidatePolicyMutation(*policy); err != nil {
			return err
		}
	}

	return nil
}

func ValidateVariables(p *kyverno.ClusterPolicy, backgroundMode bool) error {
	vars := hasVariables(p)
	if len(vars) == 0 {
		return nil
	}

	if err := hasInvalidVariables(p, backgroundMode); err != nil {
		return fmt.Errorf("policy contains invalid variables: %s", err.Error())
	}

	if backgroundMode {
		if err := containsUserVariables(p, vars); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule: %s ", err)
		}
	}

	return nil
}

// hasInvalidVariables - checks for unexpected variables in the policy
func hasInvalidVariables(policy *kyverno.ClusterPolicy, background bool) error {
	for _, r := range policy.Spec.Rules {
		ruleCopy := r.DeepCopy()

		if err := ruleForbiddenSectionsHaveVariables(ruleCopy); err != nil {
			return err
		}

		// skip variable checks on verifyImages.attestations, as variables in attestations are dynamic
		for _, vi := range ruleCopy.VerifyImages {
			for _, a := range vi.Attestations {
				a.Conditions = nil
			}
		}

		ctx := buildContext(ruleCopy, background)
		if _, err := variables.SubstituteAllInRule(log.Log, ctx, *ruleCopy); !checkNotFoundErr(err) {
			return fmt.Errorf("variable substitution failed for rule %s: %s", ruleCopy.Name, err.Error())
		}
	}

	return nil
}

// for now forbidden sections are match, exclude and
func ruleForbiddenSectionsHaveVariables(rule *kyverno.Rule) error {
	var err error

	err = jsonPatchPathHasVariables(rule.Mutation.PatchesJSON6902)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in patchesJSON6902 path section", rule.Name)
	}

	err = objectHasVariables(rule.ExcludeResources)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in exclude section", rule.Name)
	}

	err = objectHasVariables(rule.MatchResources)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in match section", rule.Name)
	}

	return nil
}

// hasVariables - check for variables in the policy
func hasVariables(policy *kyverno.ClusterPolicy) [][]string {
	policyRaw, _ := json.Marshal(policy)
	matches := variables.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

func jsonPatchPathHasVariables(patch string) error {
	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		path, err := operation.Path()
		if err != nil {
			return err
		}

		vars := variables.RegexVariables.FindAllString(path, -1)
		if len(vars) > 0 {
			return fmt.Errorf("operation \"%s\" has forbidden variables", operation.Kind())
		}
	}

	return nil
}

func objectHasVariables(object interface{}) error {
	var err error
	objectJSON, err := json.Marshal(object)
	if err != nil {
		return err
	}

	if len(common.RegexVariables.FindAllStringSubmatch(string(objectJSON), -1)) > 0 {
		return fmt.Errorf("invalid variables")
	}

	return nil
}

func buildContext(rule *kyverno.Rule, background bool) *context.MockContext {
	re := getAllowedVariables(background)
	ctx := context.NewMockContext(re)

	addContextVariables(rule.Context, ctx)

	for _, fe := range rule.Validation.ForEachValidation {
		addContextVariables(fe.Context, ctx)
	}

	for _, fe := range rule.Mutation.ForEachMutation {
		addContextVariables(fe.Context, ctx)
	}

	return ctx
}

func getAllowedVariables(background bool) *regexp.Regexp {
	if background {
		return allowedVariablesBackground
	}

	return allowedVariables
}

func addContextVariables(entries []kyverno.ContextEntry, ctx *context.MockContext) {
	for _, contextEntry := range entries {
		if contextEntry.APICall != nil {
			ctx.AddVariable(contextEntry.Name + "*")
		}

		if contextEntry.ConfigMap != nil {
			ctx.AddVariable(contextEntry.Name + ".data.*")
		}
	}
}

func checkNotFoundErr(err error) bool {
	if err != nil {
		switch err.(type) {
		case jmespath.NotFoundError:
			return true
		case context.InvalidVariableErr:
			return false
		default:
			return false
		}
	}

	return true
}

func validateElementInForEach(document apiextensions.JSON) error {
	jsonByte, err := json.Marshal(document)
	if err != nil {
		return err
	}

	var jsonInterface interface{}
	err = json.Unmarshal(jsonByte, &jsonInterface)
	if err != nil {
		return err
	}
	_, err = variables.ValidateElementInForEach(log.Log, jsonInterface)
	return err
}

func validateMatchKindHelper(rule kyverno.Rule) error {
	if !ruleOnlyDealsWithResourceMetaData(rule) {
		return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
			" the rule does not match any kind")
	}

	return fmt.Errorf("at least one element must be specified in a kind block, the kind attribute is mandatory when working with the resources element")
}

// doMatchAndExcludeConflict checks if the resultant
// of match and exclude block is not an empty set
// returns true if it is an empty set
func doMatchAndExcludeConflict(rule kyverno.Rule) bool {

	if len(rule.ExcludeResources.All) > 0 || len(rule.MatchResources.All) > 0 {
		return false
	}

	// if both have any then no resource should be common
	if len(rule.MatchResources.Any) > 0 && len(rule.ExcludeResources.Any) > 0 {
		for _, rmr := range rule.MatchResources.Any {
			for _, rer := range rule.ExcludeResources.Any {
				if reflect.DeepEqual(rmr, rer) {
					return true
				}
			}
		}
		return false
	}

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

	if len(rule.ExcludeResources.ResourceDescription.Names) > 0 {
		excludeSlice := rule.ExcludeResources.ResourceDescription.Names
		matchSlice := rule.MatchResources.ResourceDescription.Names

		// if exclude block has something and match doesn't it means we
		// have a non empty set
		if len(rule.MatchResources.ResourceDescription.Names) == 0 {
			return false
		}

		// if *any* name in match and exclude conflicts
		// we want user to fix that
		for _, matchName := range matchSlice {
			for _, excludeName := range excludeSlice {
				if wildcard.Match(excludeName, matchName) {
					return true
				}
			}
		}
		return false
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

	patternMapMutate, _ := rule.Mutation.PatchStrategicMerge.(map[string]interface{})
	for k := range patternMapMutate {
		if k != "metadata" {
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

	if (len(rule.MatchResources.Any) > 0 || len(rule.MatchResources.All) > 0) && !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyverno.ResourceDescription{}) {
		return "match.", fmt.Errorf("can't specify any/all together with match resources")
	}

	if (len(rule.ExcludeResources.Any) > 0 || len(rule.ExcludeResources.All) > 0) && !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyverno.ResourceDescription{}) {
		return "exclude.", fmt.Errorf("can't specify any/all together with exclude resources")
	}

	if len(rule.MatchResources.Any) > 0 && len(rule.MatchResources.All) > 0 {
		return "match.", fmt.Errorf("can't specify any and all together")
	}

	if len(rule.ExcludeResources.Any) > 0 && len(rule.ExcludeResources.All) > 0 {
		return "match.", fmt.Errorf("can't specify any and all together")
	}

	if len(rule.MatchResources.Any) > 0 {
		for _, rmr := range rule.MatchResources.Any {
			// matched resources
			if path, err := validateMatchedResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("match.resources.%s", path), err
			}
		}
	} else if len(rule.MatchResources.All) > 0 {
		for _, rmr := range rule.MatchResources.All {
			// matched resources
			if path, err := validateMatchedResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("match.resources.%s", path), err
			}
		}
	} else {
		// matched resources
		if path, err := validateMatchedResourceDescription(rule.MatchResources.ResourceDescription); err != nil {
			return fmt.Sprintf("match.resources.%s", path), err
		}
	}

	if len(rule.ExcludeResources.Any) > 0 {
		for _, rmr := range rule.ExcludeResources.Any {
			// exclude resources
			if path, err := validateExcludeResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("exclude.resources.%s", path), err
			}
		}
	} else if len(rule.ExcludeResources.All) > 0 {
		for _, rmr := range rule.ExcludeResources.All {
			// exclude resources
			if path, err := validateExcludeResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("exclude.resources.%s", path), err
			}
		}
	} else {
		// exclude resources
		if path, err := validateExcludeResourceDescription(rule.ExcludeResources.ResourceDescription); err != nil {
			return fmt.Sprintf("exclude.resources.%s", path), err
		}
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
	if c.Key == nil || c.Value == nil || c.Operator == "" {
		return "", fmt.Errorf("entered value of `key`, `value` or `operator` is missing or misspelled")
	}
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
		return "value", fmt.Errorf("'value' field found to be of the type %v. The provided value/values are expected to be either in the form of a string or list", reflect.TypeOf(c.Value).Kind())
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
	ruleTypes := []bool{r.HasMutate(), r.HasValidate(), r.HasGenerate(), r.HasVerifyImages()}

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
		return fmt.Errorf("no operation defined in the rule '%s'.(supported operations: mutate,validate,generate,verifyImages)", r.Name)
	} else if operationCount != 1 {
		return fmt.Errorf("multiple operations defined in the rule '%s', only one operation (mutate,validate,generate,verifyImages) is allowed per rule", r.Name)
	}
	return nil
}

func validateRuleContext(rule kyverno.Rule) error {
	if rule.Context == nil || len(rule.Context) == 0 {
		return nil
	}

	contextNames := make([]string, 0)

	for _, entry := range rule.Context {
		if entry.Name == "" {
			return fmt.Errorf("a name is required for context entries")
		}
		contextNames = append(contextNames, entry.Name)

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

	ruleBytes, _ := json.Marshal(rule)
	ruleString := strings.ReplaceAll(string(ruleBytes), " ", "")
	for _, contextName := range contextNames {
		if !strings.Contains(ruleString, fmt.Sprintf("{{"+contextName)) && !strings.Contains(ruleString, fmt.Sprintf("{{\\\""+contextName)) {
			return fmt.Errorf("context variable `%s` is not used in the policy", contextName)
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

	// Replace all variables to prevent validation failing on variable keys.
	urlPath := variables.ReplaceAllVars(entry.APICall.URLPath, func(s string) string { return "kyvernoapicallvariable" })

	if _, err := engine.NewAPIPath(urlPath); err != nil {
		return err
	}

	// If JMESPath contains variables, the validation will fail because it's not possible to infer which value
	// will be inserted by the variable
	// Skip validation if a variable is detected

	jmesPath := variables.ReplaceAllVars(entry.APICall.JMESPath, func(s string) string { return "kyvernojmespathvariable" })

	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.APICall.JMESPath != "" {
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

	if rd.Name != "" && len(rd.Names) > 0 {
		return "", fmt.Errorf("both name and names can not be specified together")
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

	if rd.Name != "" && len(rd.Names) > 0 {
		return "", fmt.Errorf("both name and names can not be specified together")
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
func checkClusterResourceInMatchAndExclude(rule kyverno.Rule, clusterResources []string, mock bool, res []*metav1.APIResourceList) error {
	// Contains Namespaces in Match->ResourceDescription
	if len(rule.MatchResources.ResourceDescription.Namespaces) > 0 {
		return fmt.Errorf("namespaced cluster policy : field namespaces not allowed in match.resources")
	}
	// Contains Namespaces in Exclude->ResourceDescription
	if len(rule.ExcludeResources.ResourceDescription.Namespaces) > 0 {
		return fmt.Errorf("namespaced cluster policy : field namespaces not allowed in exclude.resources")
	}

	if !mock {
		// Contains "Cluster Wide Resources" in Match->ResourceDescription->Kinds
		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			for _, k := range clusterResources {
				if kind == k {
					return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in match.resources.kinds", kind)
				}
			}
		}

		// Contains "Cluster Wide Resources" in Match->All->ResourceFilter->ResourceDescription->Kinds
		for _, allResourceFilter := range rule.MatchResources.All {
			fmt.Println(allResourceFilter.ResourceDescription)
			for _, kind := range allResourceFilter.ResourceDescription.Kinds {
				for _, k := range clusterResources {
					if kind == k {
						return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in match.resources.kinds", kind)
					}
				}
			}
		}

		// Contains "Cluster Wide Resources" in Match->Any->ResourceFilter->ResourceDescription->Kinds
		for _, allResourceFilter := range rule.MatchResources.Any {
			fmt.Println(allResourceFilter.ResourceDescription)
			for _, kind := range allResourceFilter.ResourceDescription.Kinds {
				for _, k := range clusterResources {
					if kind == k {
						return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in match.resources.kinds", kind)
					}
				}
			}
		}

		// Contains "Cluster Wide Resources" in Exclude->ResourceDescription->Kinds
		for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
			for _, k := range clusterResources {
				if kind == k {
					return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in exclude.resources.kinds", kind)
				}
			}

		}

		// Contains "Cluster Wide Resources" in Exclude->All->ResourceFilter->ResourceDescription->Kinds
		for _, allResourceFilter := range rule.ExcludeResources.All {
			fmt.Println(allResourceFilter.ResourceDescription)
			for _, kind := range allResourceFilter.ResourceDescription.Kinds {
				for _, k := range clusterResources {
					if kind == k {
						return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in match.resources.kinds", kind)
					}
				}
			}
		}

		// Contains "Cluster Wide Resources" in Exclude->Any->ResourceFilter->ResourceDescription->Kinds
		for _, allResourceFilter := range rule.ExcludeResources.Any {
			fmt.Println(allResourceFilter.ResourceDescription)
			for _, kind := range allResourceFilter.ResourceDescription.Kinds {
				for _, k := range clusterResources {
					if kind == k {
						return fmt.Errorf("namespaced policy : cluster-wide resource '%s' not allowed in match.resources.kinds", kind)
					}
				}
			}
		}

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
						} else {
							if rule.Generation.Namespace != "" {
								return fmt.Errorf("path: spec.rules[%v]: do not mention the namespace to generate a non namespaced resource", rule.Name)
							}
						}
					}
				}
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

func validateKinds(kinds []string, mock bool, client *dclient.Client, p kyverno.ClusterPolicy) error {
	for _, kind := range kinds {
		_, k := comn.GetKindFromGVK(kind)
		if k == p.Kind {
			return fmt.Errorf("kind and match resource kind should not be the same")
		}
	}
	return nil
}

func validateVerifyImagesRule(i *kyverno.ImageVerification) error {
	hasKey := i.Key != ""
	hasRoots := i.Roots != ""
	hasSubject := i.Subject != ""

	if (hasKey && !hasRoots && !hasSubject) || (hasRoots && hasSubject) {
		return nil
	}

	return fmt.Errorf("either a public key, or root certificates and an email, are required")
}
