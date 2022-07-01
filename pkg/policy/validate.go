package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/distribution/distribution/reference"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/autogen"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var allowedVariables = regexp.MustCompile(`request\.|serviceAccountName|serviceAccountNamespace|element|elementIndex|@|images\.|target\.|([a-z_0-9]+\()[^{}]`)

var allowedVariablesBackground = regexp.MustCompile(`request\.|element|elementIndex|@|images\.|target\.|([a-z_0-9]+\()[^{}]`)

// wildCardAllowedVariables represents regex for the allowed fields in wildcards
var wildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace)[^{}]*\}\}`)

var errOperationForbidden = errors.New("variables are forbidden in the path of a JSONPatch")

// validateJSONPatchPathForForwardSlash checks for forward slash
func validateJSONPatchPathForForwardSlash(patch string) error {
	// Replace all variables in PatchesJSON6902, all variable checks should have happened already.
	// This prevents further checks from failing unexpectedly.
	patch = variables.ReplaceAllVars(patch, func(s string) string { return "kyvernojsonpatchvariable" })

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
func Validate(policy kyverno.PolicyInterface, client dclient.Interface, mock bool, openAPIController *openapi.Controller) (*admissionv1.AdmissionResponse, error) {
	namespaced := policy.IsNamespaced()
	spec := policy.GetSpec()
	background := spec.BackgroundProcessingEnabled()
	onPolicyUpdate := spec.GetMutateExistingOnPolicyUpdate()

	var errs field.ErrorList
	specPath := field.NewPath("spec")

	err := ValidateVariables(policy, background)
	if err != nil {
		return nil, err
	}

	if onPolicyUpdate {
		err := ValidateOnPolicyUpdate(policy, onPolicyUpdate)
		if err != nil {
			return nil, err
		}
	}

	var res []*metav1.APIResourceList
	clusterResources := sets.NewString()
	if !mock && namespaced {
		// Get all the cluster type kind supported by cluster
		res, err := discovery.ServerPreferredResources(client.Discovery().DiscoveryInterface())
		if err != nil {
			return nil, err
		}
		for _, resList := range res {
			for _, r := range resList.APIResources {
				if !r.Namespaced {
					clusterResources.Insert(r.Kind)
				}
			}
		}
	}

	if errs := policy.Validate(clusterResources); len(errs) != 0 {
		return nil, errs.ToAggregate()
	}
	rules := autogen.ComputeRules(policy)
	rulesPath := specPath.Child("rules")
	for i, rule := range rules {
		rulePath := rulesPath.Index(i)
		//check for forward slash
		if err := validateJSONPatchPathForForwardSlash(rule.Mutation.PatchesJSON6902); err != nil {
			return nil, fmt.Errorf("path must begin with a forward slash: spec.rules[%d]: %s", i, err)
		}

		if jsonPatchOnPod(rule) {
			msg := "Pods managed by workload controllers should not be directly mutated using policies. " +
				"Use the autogen feature or write policies that match Pod controllers."
			log.Log.V(1).Info(msg)
			return &admissionv1.AdmissionResponse{
				Allowed:  true,
				Warnings: []string{msg},
			}, nil
		}

		// validate resource description
		if path, err := validateResources(rulePath, rule); err != nil {
			return nil, fmt.Errorf("path: spec.rules[%d].%s: %v", i, path, err)
		}

		err := validateElementInForEach(rule)
		if err != nil {
			return nil, err
		}

		if err := validateRuleContext(rule); err != nil {
			return nil, fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		// validate Cluster Resources in namespaced policy
		// For namespaced policy, ClusterResource type field and values are not allowed in match and exclude
		if namespaced {
			return nil, checkClusterResourceInMatchAndExclude(rule, clusterResources, mock, res)
		}

		// validate rule actions
		// - Mutate
		// - Validate
		// - Generate
		if err := validateActions(i, &rules[i], client, mock); err != nil {
			return nil, err
		}

		// If a rule's match block does not match any kind,
		// we should only allow it to have metadata in its overlay
		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				if len(rmr.Kinds) == 0 {
					return nil, validateMatchKindHelper(rule)
				}
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				if len(rmr.Kinds) == 0 {
					return nil, validateMatchKindHelper(rule)
				}
			}
		} else {
			if len(rule.MatchResources.Kinds) == 0 {
				return nil, validateMatchKindHelper(rule)
			}
		}

		if utils.ContainsString(rule.MatchResources.Kinds, "*") && spec.BackgroundProcessingEnabled() {
			return nil, fmt.Errorf("wildcard policy not allowed in background mode. Set spec.background=false to disable background mode for this policy rule ")
		}

		if (utils.ContainsString(rule.MatchResources.Kinds, "*") && len(rule.MatchResources.Kinds) > 1) || (utils.ContainsString(rule.ExcludeResources.Kinds, "*") && len(rule.ExcludeResources.Kinds) > 1) {
			return nil, fmt.Errorf("wildard policy can not deal more than one kind")
		}

		if utils.ContainsString(rule.MatchResources.Kinds, "*") || utils.ContainsString(rule.ExcludeResources.Kinds, "*") {

			if rule.HasGenerate() || rule.HasVerifyImages() || rule.Validation.ForEachValidation != nil {
				return nil, fmt.Errorf("wildcard policy does not support rule type")
			}

			if rule.HasValidate() {

				if rule.Validation.GetPattern() != nil || rule.Validation.GetAnyPattern() != nil {
					if !ruleOnlyDealsWithResourceMetaData(rule) {
						return nil, fmt.Errorf("policy can only deal with the metadata field of the resource if" +
							" the rule does not match any kind")
					}
				}

				if rule.Validation.Deny != nil {
					kyvernoConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.Validation.Deny.GetAnyAllConditions())
					switch typedConditions := kyvernoConditions.(type) {
					case []kyverno.Condition: // backwards compatibility
						for _, condition := range typedConditions {
							key := condition.GetKey()
							if !strings.Contains(key.(string), "request.object.metadata.") && (!wildCardAllowedVariables.MatchString(key.(string)) || strings.Contains(key.(string), "request.object.spec")) {
								return nil, fmt.Errorf("policy can only deal with the metadata field of the resource if" +
									" the rule does not match any kind")
							}
						}
					}
				}
			}

			if rule.HasMutate() {
				if !ruleOnlyDealsWithResourceMetaData(rule) {
					return nil, fmt.Errorf("policy can only deal with the metadata field of the resource if" +
						" the rule does not match any kind")
				}
			}

			if rule.HasVerifyImages() {
				verifyImagePath := rulePath.Child("verifyImages")
				for index, i := range rule.VerifyImages {
					errs = append(errs, i.Validate(verifyImagePath.Index(index))...)
				}
			}

			if len(errs) != 0 {
				return nil, errs.ToAggregate()
			}
		}

		var podOnlyMap = make(map[string]bool) //Validate that Kind is only Pod
		podOnlyMap["Pod"] = true
		if reflect.DeepEqual(common.GetKindsFromRule(rule), podOnlyMap) && podControllerAutoGenExclusion(policy) {
			msg := "Policies that match Pods apply to all Pods including those created and managed by controllers " +
				"excluded from autogen. Use preconditions to exclude the Pods managed by controllers which are " +
				"excluded from autogen. Refer to https://kyverno.io/docs/writing-policies/autogen/ for details."

			return &admissionv1.AdmissionResponse{
				Allowed:  true,
				Warnings: []string{msg},
			}, nil
		}

		//Validate Kind with match resource kinds
		match := rule.MatchResources
		exclude := rule.ExcludeResources
		for _, value := range match.Any {
			if !utils.ContainsString(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, client, policy)
				if err != nil {
					return nil, errors.Wrapf(err, "the kind defined in the any match resource is invalid")
				}
			}
		}
		for _, value := range match.All {
			if !utils.ContainsString(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, client, policy)
				if err != nil {
					return nil, errors.Wrapf(err, "the kind defined in the all match resource is invalid")
				}
			}
		}
		for _, value := range exclude.Any {
			if !utils.ContainsString(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, client, policy)
				if err != nil {
					return nil, errors.Wrapf(err, "the kind defined in the any exclude resource is invalid")
				}
			}
		}
		for _, value := range exclude.All {
			if !utils.ContainsString(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, client, policy)
				if err != nil {
					return nil, errors.Wrapf(err, "the kind defined in the all exclude resource is invalid")
				}
			}
		}
		if !utils.ContainsString(rule.MatchResources.Kinds, "*") {
			err := validateKinds(rule.MatchResources.Kinds, mock, client, policy)
			if err != nil {
				return nil, errors.Wrapf(err, "match resource kind is invalid")
			}
			err = validateKinds(rule.ExcludeResources.Kinds, mock, client, policy)
			if err != nil {
				return nil, errors.Wrapf(err, "exclude resource kind is invalid")
			}
		}

		// Validate string values in labels
		if !isLabelAndAnnotationsString(rule) {
			return nil, fmt.Errorf("labels and annotations supports only string values, \"use double quotes around the non string values\"")
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

	if spec.SchemaValidation == nil || *spec.SchemaValidation {
		if err := openAPIController.ValidatePolicyMutation(policy); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func ValidateVariables(p kyverno.PolicyInterface, backgroundMode bool) error {
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
func hasInvalidVariables(policy kyverno.PolicyInterface, background bool) error {
	for _, r := range autogen.ComputeRules(policy) {
		ruleCopy := r.DeepCopy()

		if err := ruleForbiddenSectionsHaveVariables(ruleCopy); err != nil {
			return err
		}

		// skip variable checks on verifyImages.attestations, as variables in attestations are dynamic
		for i, vi := range ruleCopy.VerifyImages {
			for j := range vi.Attestations {
				ruleCopy.VerifyImages[i].Attestations[j].Conditions = nil
			}
		}

		ctx := buildContext(ruleCopy, background)
		if _, err := variables.SubstituteAllInRule(log.Log, ctx, *ruleCopy); !checkNotFoundErr(err) {
			return fmt.Errorf("variable substitution failed for rule %s: %s", ruleCopy.Name, err.Error())
		}
	}

	return nil
}

func ValidateOnPolicyUpdate(p kyverno.PolicyInterface, onPolicyUpdate bool) error {
	vars := hasVariables(p)
	if len(vars) == 0 {
		return nil
	}

	if err := hasInvalidVariables(p, onPolicyUpdate); err != nil {
		return fmt.Errorf("update event, policy contains invalid variables: %s", err.Error())
	}

	if err := containsUserVariables(p, vars); err != nil {
		return fmt.Errorf("only select variables are allowed in on policy update. Set spec.mutateExistingOnPolicyUpdate=false to disable update policy mode for this policy rule: %s ", err)
	}

	return nil
}

// for now forbidden sections are match, exclude and
func ruleForbiddenSectionsHaveVariables(rule *kyverno.Rule) error {
	var err error

	err = jsonPatchPathHasVariables(rule.Mutation.PatchesJSON6902)
	if err != nil && errors.Is(errOperationForbidden, err) {
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
func hasVariables(policy kyverno.PolicyInterface) [][]string {
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
			return errOperationForbidden
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
		if contextEntry.APICall != nil || contextEntry.ImageRegistry != nil || contextEntry.Variable != nil {
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

	patternMap, ok := rule.Validation.GetPattern().(map[string]interface{})
	if ok {
		return checkMetadata(patternMap)
	} else if rule.Validation.GetAnyPattern() != nil {
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
	patches, _ := rule.Mutation.GetPatchStrategicMerge().(map[string]interface{})
	for k := range patches {
		if k != "metadata" {
			return false
		}
	}

	if rule.Mutation.PatchesJSON6902 != "" {
		bytes := []byte(rule.Mutation.PatchesJSON6902)
		jp, _ := jsonpatch.DecodePatch(bytes)
		for _, o := range jp {
			path, _ := o.Path()
			if !strings.HasPrefix(path, "/metadata") {
				return false
			}
		}
	}

	patternMap, _ := rule.Validation.GetPattern().(map[string]interface{})
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

func validateResources(path *field.Path, rule kyverno.Rule) (string, error) {
	// validate userInfo in match and exclude
	if errs := rule.ExcludeResources.UserInfo.Validate(path.Child("exclude")); len(errs) != 0 {
		return "exclude", errs.ToAggregate()
	}

	if (len(rule.MatchResources.Any) > 0 || len(rule.MatchResources.All) > 0) && !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyverno.ResourceDescription{}) {
		return "match.", fmt.Errorf("can't specify any/all together with match resources")
	}

	if (len(rule.ExcludeResources.Any) > 0 || len(rule.ExcludeResources.All) > 0) && !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyverno.ResourceDescription{}) {
		return "exclude.", fmt.Errorf("can't specify any/all together with exclude resources")
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

	//validating the values present under validate.preconditions, if they exist
	if target := rule.GetAnyAllConditions(); target != nil {
		if path, err := validateConditions(target, "preconditions"); err != nil {
			return fmt.Sprintf("validate.%s", path), err
		}
	}
	//validating the values present under validate.conditions, if they exist
	if rule.Validation.Deny != nil {
		if target := rule.Validation.Deny.GetAnyAllConditions(); target != nil {
			if path, err := validateConditions(target, "conditions"); err != nil {
				return fmt.Sprintf("validate.deny.%s", path), err
			}
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
	k := c.GetKey()
	v := c.GetValue()
	if k == nil || v == nil || c.Operator == "" {
		return "", fmt.Errorf("entered value of `key`, `value` or `operator` is missing or misspelled")
	}
	switch reflect.TypeOf(k).Kind() {
	case reflect.String:
		value, err := validateValuesKeyRequest(c)
		return value, err
	default:
		return "", nil
	}
}

func validateValuesKeyRequest(c kyverno.Condition) (string, error) {
	k := c.GetKey()
	switch strings.ReplaceAll(k.(string), " ", "") {
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
	v := c.GetValue()
	switch reflect.TypeOf(v).Kind() {
	case reflect.String:
		valueStr := v.(string)
		// allow templatized values like {{ config-map.data.sample-key }}
		// because they might be actually pointing to a rightful value in the provided config-map
		if len(valueStr) >= 4 && valueStr[:2] == "{{" && valueStr[len(valueStr)-2:] == "}}" {
			return "", nil
		}
		if !valuesAllowed[valueStr] {
			return fmt.Sprintf("value: %s", v.(string)), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", v.(string))
		}
	case reflect.Slice:
		values := reflect.ValueOf(v)
		for i := 0; i < values.Len(); i++ {
			value := values.Index(i).Interface().(string)
			if !valuesAllowed[value] {
				return fmt.Sprintf("value[%d]", i), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", value)
			}
		}
	default:
		return "value", fmt.Errorf("'value' field found to be of the type %v. The provided value/values are expected to be either in the form of a string or list", reflect.TypeOf(v).Kind())
	}
	return "", nil
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
		for _, v := range []string{"images", "request", "serviceAccountName", "serviceAccountNamespace", "element", "elementIndex"} {
			if entry.Name == v || strings.HasPrefix(entry.Name, v+".") {
				return fmt.Errorf("entry name %s is invalid as it conflicts with a pre-defined variable %s", entry.Name, v)
			}
		}
		contextNames = append(contextNames, entry.Name)

		var err error
		if entry.ConfigMap != nil && entry.APICall == nil && entry.ImageRegistry == nil && entry.Variable == nil {
			err = validateConfigMap(entry)
		} else if entry.ConfigMap == nil && entry.APICall != nil && entry.ImageRegistry == nil && entry.Variable == nil {
			err = validateAPICall(entry)
		} else if entry.ConfigMap == nil && entry.APICall == nil && entry.ImageRegistry != nil && entry.Variable == nil {
			err = validateImageRegistry(entry)
		} else if entry.ConfigMap == nil && entry.APICall == nil && entry.ImageRegistry == nil && entry.Variable != nil {
			err = validateVariable(entry)
		} else {
			return fmt.Errorf("exactly one of configMap or apiCall or imageRegistry or variable is required for context entries")
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func validateVariable(entry kyverno.ContextEntry) error {
	// If JMESPath contains variables, the validation will fail because it's not possible to infer which value
	// will be inserted by the variable
	// Skip validation if a variable is detected
	jmesPath := variables.ReplaceAllVars(entry.Variable.JMESPath, func(s string) string { return "kyvernojmespathvariable" })
	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.Variable.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.Variable.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.Variable.JMESPath, err)
		}
	}
	if entry.Variable.Value == nil && jmesPath == "" {
		return fmt.Errorf("a variable must define a value or a jmesPath expression")
	}
	if entry.Variable.Default != nil && jmesPath == "" {
		return fmt.Errorf("a variable must define a default value only when a jmesPath expression is defined")
	}
	return nil
}

func validateConfigMap(entry kyverno.ContextEntry) error {
	if entry.ConfigMap.Name == "" {
		return fmt.Errorf("a name is required for configMap context entry")
	}

	if entry.ConfigMap.Namespace == "" {
		return fmt.Errorf("a namespace is required for configMap context entry")
	}

	return nil
}

func validateAPICall(entry kyverno.ContextEntry) error {
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

func validateImageRegistry(entry kyverno.ContextEntry) error {
	if entry.ImageRegistry.Reference == "" {
		return fmt.Errorf("a ref is required for imageRegistry context entry")
	}
	// Replace all variables to prevent validation failing on variable keys.
	ref := variables.ReplaceAllVars(entry.ImageRegistry.Reference, func(s string) string { return "kyvernoimageref" })

	// it's no use validating a refernce that contains a variable
	if !strings.Contains(ref, "kyvernoimageref") {
		_, err := reference.Parse(ref)
		if err != nil {
			return errors.Wrapf(err, "bad image: %s", ref)
		}
	}

	// If JMESPath contains variables, the validation will fail because it's not possible to infer which value
	// will be inserted by the variable
	// Skip validation if a variable is detected
	jmesPath := variables.ReplaceAllVars(entry.ImageRegistry.JMESPath, func(s string) string { return "kyvernojmespathvariable" })

	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.ImageRegistry.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.ImageRegistry.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.ImageRegistry.JMESPath, err)
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

	return "", nil
}

// checkClusterResourceInMatchAndExclude returns false if namespaced ClusterPolicy contains cluster wide resources in
// Match and Exclude block
func checkClusterResourceInMatchAndExclude(rule kyverno.Rule, clusterResources sets.String, mock bool, res []*metav1.APIResourceList) error {
	if !mock {
		// Check for generate policy
		// - if resource to be generated is namespaced resource then the namespace field
		// should be mentioned
		// - if resource to be generated is non namespaced resource then the namespace field
		// should not be mentioned
		if rule.HasGenerate() {
			generateResourceKind := rule.Generation.Kind
			generateResourceAPIVersion := rule.Generation.APIVersion
			for _, resList := range res {
				for _, r := range resList.APIResources {
					if r.Kind == generateResourceKind && (len(generateResourceAPIVersion) == 0 || r.Version == generateResourceAPIVersion) {
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

func podControllerAutoGenExclusion(policy kyverno.PolicyInterface) bool {
	annotations := policy.GetAnnotations()
	val, ok := annotations[kyverno.PodControllersAnnotation]
	if !ok || val == "none" {
		return false
	}

	reorderVal := strings.Split(strings.ToLower(val), ",")
	sort.Slice(reorderVal, func(i, j int) bool { return reorderVal[i] < reorderVal[j] })
	if ok && reflect.DeepEqual(reorderVal, []string{"cronjob", "daemonset", "deployment", "job", "statefulset"}) == false {
		return true
	}
	return false
}

// validateKinds verifies if an API resource that matches 'kind' is valid kind
// and found in the cache, returns error if not found
func validateKinds(kinds []string, mock bool, client dclient.Interface, p kyverno.PolicyInterface) error {
	for _, kind := range kinds {
		gv, k := kubeutils.GetKindFromGVK(kind)
		if k == p.GetKind() {
			return fmt.Errorf("kind and match resource kind should not be the same")
		}

		if !mock && !kubeutils.SkipSubResources(k) && !strings.Contains(kind, "*") {
			_, _, err := client.Discovery().FindResource(gv, k)
			if err != nil {
				return fmt.Errorf("unable to convert GVK to GVR for kinds %s, err: %s", kinds, err)
			}
		}
	}
	return nil
}
