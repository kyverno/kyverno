package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/distribution/distribution/reference"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/jmespath/go-jmespath"
	"github.com/jmoiron/jsonq"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	openapicontroller "github.com/kyverno/kyverno/pkg/controllers/openapi"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/openapi"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
)

var allowedVariables = regexp.MustCompile(`request\.|serviceAccountName|serviceAccountNamespace|element|elementIndex|@|images\.|image\.|target\.|([a-z_0-9]+\()[^{}]`)

var allowedVariablesBackground = regexp.MustCompile(`request\.|element|elementIndex|@|images\.|image\.|target\.|([a-z_0-9]+\()[^{}]`)

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

func validateJSONPatch(patch string, ruleIdx int) error {
	patch = variables.ReplaceAllVars(patch, func(s string) string { return "kyvernojsonpatchvariable" })

	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		op := operation.Kind()
		if op != "add" && op != "remove" && op != "replace" {
			return fmt.Errorf("Unexpected kind: spec.rules[%d]: %s", ruleIdx, op)
		}
		v, _ := operation.ValueInterface()
		if v != nil {
			vs := fmt.Sprintf("%v", v)
			if strings.ContainsAny(vs, `"`) || strings.ContainsAny(vs, `'`) {
				return fmt.Errorf("missing quote around value: spec.rules[%d]: %s", ruleIdx, vs)
			}
		}
	}

	return nil
}

func checkValidationFailureAction(spec *kyvernov1.Spec) []string {
	msg := "Validation failure actions enforce/audit are deprecated, use Enforce/Audit instead."
	if spec.ValidationFailureAction == "enforce" || spec.ValidationFailureAction == "audit" {
		return []string{msg}
	}
	for _, override := range spec.ValidationFailureActionOverrides {
		if override.Action == "enforce" || override.Action == "audit" {
			return []string{msg}
		}
	}
	return nil
}

// Validate checks the policy and rules declarations for required configurations
func Validate(policy kyvernov1.PolicyInterface, client dclient.Interface, mock bool, openApiManager openapi.Manager) ([]string, error) {
	var warnings []string
	namespaced := policy.IsNamespaced()
	spec := policy.GetSpec()
	background := spec.BackgroundProcessingEnabled()
	onPolicyUpdate := spec.GetMutateExistingOnPolicyUpdate()
	if !mock {
		openapicontroller.NewController(client, openApiManager).CheckSync(context.TODO())
	}

	warnings = append(warnings, checkValidationFailureAction(spec)...)
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	err := ValidateVariables(policy, background)
	if err != nil {
		return warnings, err
	}

	if onPolicyUpdate {
		err := ValidateOnPolicyUpdate(policy, onPolicyUpdate)
		if err != nil {
			return warnings, err
		}
	}

	var res []*metav1.APIResourceList
	clusterResources := sets.New[string]()
	if !mock && namespaced {
		// Get all the cluster type kind supported by cluster
		res, err = discovery.ServerPreferredResources(client.Discovery().DiscoveryInterface())
		if err != nil {
			if discovery.IsGroupDiscoveryFailedError(err) {
				err := err.(*discovery.ErrGroupDiscoveryFailed)
				for gv, err := range err.Groups {
					logging.Error(err, "failed to list api resources", "group", gv)
				}
			} else {
				return warnings, err
			}
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
		return warnings, errs.ToAggregate()
	}

	if !namespaced {
		err := validateNamespaces(spec, specPath.Child("validationFailureActionOverrides"))
		if err != nil {
			return warnings, err
		}
	}

	rules := autogen.ComputeRules(policy)
	rulesPath := specPath.Child("rules")
	for i, rule := range rules {
		rulePath := rulesPath.Index(i)
		// check for forward slash
		if err := validateJSONPatchPathForForwardSlash(rule.Mutation.PatchesJSON6902); err != nil {
			return warnings, fmt.Errorf("path must begin with a forward slash: spec.rules[%d]: %s", i, err)
		}
		if err := validateJSONPatch(rule.Mutation.PatchesJSON6902, i); err != nil {
			return warnings, fmt.Errorf("%s", err)
		}

		if jsonPatchOnPod(rule) {
			msg := "Pods managed by workload controllers should not be directly mutated using policies. " +
				"Use the autogen feature or write policies that match Pod controllers."
			logging.V(1).Info(msg)
			warnings = append(warnings, msg)
		}

		// validate resource description
		if path, err := validateResources(rulePath, rule); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d].%s: %v", i, path, err)
		}

		err := validateElementInForEach(rule)
		if err != nil {
			return warnings, err
		}

		if err := validateRuleContext(rule); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		// If a rule's match block does not match any kind,
		// we should only allow it to have metadata in its overlay
		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				if len(rmr.Kinds) == 0 {
					return warnings, validateMatchKindHelper(rule)
				}
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				if len(rmr.Kinds) == 0 {
					return warnings, validateMatchKindHelper(rule)
				}
			}
		} else {
			if len(rule.MatchResources.Kinds) == 0 {
				return warnings, validateMatchKindHelper(rule)
			}
		}
		// validate Cluster Resources in namespaced policy
		// For namespaced policy, ClusterResource type field and values are not allowed in match and exclude
		if namespaced {
			return warnings, checkClusterResourceInMatchAndExclude(rule, clusterResources, policy.GetNamespace(), mock, res)
		}

		// validate rule actions
		// - Mutate
		// - Validate
		// - Generate
		if err := validateActions(i, &rules[i], client, mock); err != nil {
			return warnings, err
		}

		if rule.HasVerifyImages() {
			verifyImagePath := rulePath.Child("verifyImages")
			for index, i := range rule.VerifyImages {
				errs = append(errs, i.Validate(verifyImagePath.Index(index))...)
			}
			if len(errs) != 0 {
				return warnings, errs.ToAggregate()
			}
		}

		kindsFromRule := rule.MatchResources.GetKinds()
		resourceTypesMap := make(map[string]bool)
		for _, kind := range kindsFromRule {
			_, k := kubeutils.GetKindFromGVK(kind)
			k, _ = kubeutils.SplitSubresource(k)
			resourceTypesMap[k] = true
		}
		if len(resourceTypesMap) == 1 {
			for k := range resourceTypesMap {
				if k == "Pod" && podControllerAutoGenExclusion(policy) {
					msg := "Policies that match Pods apply to all Pods including those created and managed by controllers " +
						"excluded from autogen. Use preconditions to exclude the Pods managed by controllers which are " +
						"excluded from autogen. Refer to https://kyverno.io/docs/writing-policies/autogen/ for details."

					warnings = append(warnings, msg)
				}
			}
		}

		// Validate Kind with match resource kinds
		match := rule.MatchResources
		exclude := rule.ExcludeResources
		for _, value := range match.Any {
			wildcardErr := validateWildcard(value.ResourceDescription.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
			if !slices.Contains(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, background, rule.HasValidate(), client)
				if err != nil {
					return warnings, errors.Wrapf(err, "the kind defined in the any match resource is invalid")
				}
			}
		}
		for _, value := range match.All {
			wildcardErr := validateWildcard(value.ResourceDescription.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
			if !slices.Contains(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, background, rule.HasValidate(), client)
				if err != nil {
					return warnings, errors.Wrapf(err, "the kind defined in the all match resource is invalid")
				}
			}
		}
		for _, value := range exclude.Any {
			wildcardErr := validateWildcard(value.ResourceDescription.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
			if !slices.Contains(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, background, rule.HasValidate(), client)
				if err != nil {
					return warnings, errors.Wrapf(err, "the kind defined in the any exclude resource is invalid")
				}
			}
		}
		for _, value := range exclude.All {
			wildcardErr := validateWildcard(value.ResourceDescription.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
			if !slices.Contains(value.ResourceDescription.Kinds, "*") {
				err := validateKinds(value.ResourceDescription.Kinds, mock, background, rule.HasValidate(), client)
				if err != nil {
					return warnings, errors.Wrapf(err, "the kind defined in the all exclude resource is invalid")
				}
			}
		}

		if !slices.Contains(rule.MatchResources.Kinds, "*") {
			err := validateKinds(rule.MatchResources.Kinds, mock, background, rule.HasValidate(), client)
			if err != nil {
				return warnings, errors.Wrapf(err, "match resource kind is invalid")
			}
			err = validateKinds(rule.ExcludeResources.Kinds, mock, background, rule.HasValidate(), client)
			if err != nil {
				return warnings, errors.Wrapf(err, "exclude resource kind is invalid")
			}
		} else {
			wildcardErr := validateWildcard(rule.MatchResources.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
			wildcardErr = validateWildcard(rule.ExcludeResources.Kinds, spec, rule)
			if wildcardErr != nil {
				return warnings, wildcardErr
			}
		}

		// Validate string values in labels
		if !isLabelAndAnnotationsString(rule) {
			return warnings, fmt.Errorf("labels and annotations supports only string values, \"use double quotes around the non string values\"")
		}

		// add label to source mentioned in policy
		if !mock && rule.Generation.Clone.Name != "" {
			obj, err := client.GetResource(context.TODO(), "", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
			if err != nil {
				logging.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
				continue
			}
			err = UpdateSourceResource(client, rule.Generation.Kind, rule.Generation.Clone.Namespace, policy.GetName(), obj)
			if err != nil {
				logging.Error(err, "failed to update source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
				continue
			}
			logging.V(4).Info("updated source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
		}
		if !mock && len(rule.Generation.CloneList.Kinds) != 0 {
			for _, kind := range rule.Generation.CloneList.Kinds {
				apiVersion, kind := kubeutils.GetKindFromGVK(kind)
				resources, err := client.ListResource(context.TODO(), apiVersion, kind, rule.Generation.CloneList.Namespace, rule.Generation.CloneList.Selector)
				if err != nil {
					logging.Error(err, fmt.Sprintf("failed to list resources %s/%s.", kind, rule.Generation.CloneList.Namespace))
					continue
				}
				for _, rName := range resources.Items {
					obj, err := client.GetResource(context.TODO(), apiVersion, kind, rule.Generation.CloneList.Namespace, rName.GetName())
					if err != nil {
						logging.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
						continue
					}
					err = UpdateSourceResource(client, kind, rule.Generation.CloneList.Namespace, policy.GetName(), obj)
					if err != nil {
						logging.Error(err, "failed to update source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
						continue
					}
				}
			}
		}

		matchKinds := match.GetKinds()
		excludeKinds := exclude.GetKinds()
		allKinds := make([]string, 0, len(matchKinds)+len(excludeKinds))
		allKinds = append(allKinds, matchKinds...)
		allKinds = append(allKinds, excludeKinds...)
		if rule.HasValidate() {
			validationJson, err := json.Marshal(rule.Validation)
			if err != nil {
				return nil, err
			}
			checkForScaleSubresource(validationJson, allKinds, &warnings)
			checkForStatusSubresource(validationJson, allKinds, &warnings)
		}

		if rule.HasMutate() {
			mutationJson, err := json.Marshal(rule.Mutation)
			targets := rule.Mutation.Targets
			for _, target := range targets {
				allKinds = append(allKinds, target.GetKind())
			}
			if err != nil {
				return nil, err
			}
			checkForScaleSubresource(mutationJson, allKinds, &warnings)
			checkForStatusSubresource(mutationJson, allKinds, &warnings)
		}
	}
	if !mock && (spec.SchemaValidation == nil || *spec.SchemaValidation) {
		if err := openApiManager.ValidatePolicyMutation(policy); err != nil {
			return warnings, fmt.Errorf("%s (you can bypass schema validation by setting `spec.schemaValidation: false`)", err)
		}
	}
	return warnings, nil
}

func UpdateSourceResource(client dclient.Interface, kind, namespace string, policyName string, obj *unstructured.Unstructured) error {
	updateSource := true
	label := obj.GetLabels()

	if len(label) == 0 {
		label = make(map[string]string)
		label["generate.kyverno.io/clone-policy-name"] = policyName
	} else {
		if label["generate.kyverno.io/clone-policy-name"] != "" {
			policyNames := label["generate.kyverno.io/clone-policy-name"]
			if !strings.Contains(policyNames, policyName) {
				policyNames = policyNames + "," + policyName
				label["generate.kyverno.io/clone-policy-name"] = policyNames
			} else {
				updateSource = false
			}
		} else {
			label["generate.kyverno.io/clone-policy-name"] = policyName
		}
	}

	if updateSource {
		logging.V(4).Info("updating existing clone source labels")
		obj.SetLabels(label)
		obj.SetResourceVersion("")

		_, err := client.UpdateResource(context.TODO(), obj.GetAPIVersion(), kind, namespace, obj, false)
		if err != nil {
			logging.Error(err, "failed to update source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			return err
		}
		logging.V(4).Info("updated source", "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
	}
	return nil
}

func ValidateVariables(p kyvernov1.PolicyInterface, backgroundMode bool) error {
	vars := hasVariables(p)
	if backgroundMode {
		if err := containsUserVariables(p, vars); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule: %s ", err)
		}
	}
	if err := hasInvalidVariables(p, backgroundMode); err != nil {
		return fmt.Errorf("policy contains invalid variables: %s", err.Error())
	}
	return nil
}

// hasInvalidVariables - checks for unexpected variables in the policy
func hasInvalidVariables(policy kyvernov1.PolicyInterface, background bool) error {
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
		if _, err := variables.SubstituteAllInRule(logging.GlobalLogger(), ctx, *ruleCopy); !variables.CheckNotFoundErr(err) {
			return fmt.Errorf("variable substitution failed for rule %s: %s", ruleCopy.Name, err.Error())
		}
	}

	return nil
}

func ValidateOnPolicyUpdate(p kyvernov1.PolicyInterface, onPolicyUpdate bool) error {
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
func ruleForbiddenSectionsHaveVariables(rule *kyvernov1.Rule) error {
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

	err = imageRefHasVariables(rule.VerifyImages)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in image reference section", rule.Name)
	}

	return nil
}

// hasVariables - check for variables in the policy
func hasVariables(policy kyvernov1.PolicyInterface) [][]string {
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

func imageRefHasVariables(verifyImages []kyvernov1.ImageVerification) error {
	for _, verifyImage := range verifyImages {
		verifyImage = *verifyImage.Convert()
		for _, imageRef := range verifyImage.ImageReferences {
			matches := variables.RegexVariables.FindAllString(imageRef, -1)
			if len(matches) > 0 {
				return fmt.Errorf("variables are not allowed in image reference")
			}
		}
	}
	return nil
}

func buildContext(rule *kyvernov1.Rule, background bool) *enginecontext.MockContext {
	re := getAllowedVariables(background)
	ctx := enginecontext.NewMockContext(re)

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

func addContextVariables(entries []kyvernov1.ContextEntry, ctx *enginecontext.MockContext) {
	for _, contextEntry := range entries {
		if contextEntry.APICall != nil || contextEntry.ImageRegistry != nil || contextEntry.Variable != nil {
			ctx.AddVariable(contextEntry.Name + "*")
		}

		if contextEntry.ConfigMap != nil {
			ctx.AddVariable(contextEntry.Name + ".data.*")
		}
	}
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
	_, err = variables.ValidateElementInForEach(logging.GlobalLogger(), jsonInterface)
	return err
}

func validateMatchKindHelper(rule kyvernov1.Rule) error {
	if !ruleOnlyDealsWithResourceMetaData(rule) {
		return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
			" the rule does not match any kind")
	}

	return fmt.Errorf("at least one element must be specified in a kind block, the kind attribute is mandatory when working with the resources element")
}

// isLabelAndAnnotationsString :- Validate if labels and annotations contains only string values
func isLabelAndAnnotationsString(rule kyvernov1.Rule) bool {
	checkLabelAnnotation := func(metaKey map[string]interface{}) bool {
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
		return true
	}

	// checkMetadata - Verify if the labels and annotations contains string value inside metadata
	checkMetadata := func(patternMap map[string]interface{}) bool {
		for k := range patternMap {
			if k == "metadata" {
				metaKey, ok := patternMap[k].(map[string]interface{})
				if ok {
					// range over metadata
					return checkLabelAnnotation(metaKey)
				}
			}
			if k == "spec" {
				metadata, _ := jsonq.NewQuery(patternMap).Object("spec", "template", "metadata")
				return checkLabelAnnotation(metadata)
			}
		}
		return true
	}

	if rule.HasValidate() {
		if rule.Validation.ForEachValidation != nil {
			for _, foreach := range rule.Validation.ForEachValidation {
				patternMap, ok := foreach.GetPattern().(map[string]interface{})
				if ok {
					return checkMetadata(patternMap)
				}
			}
		} else {
			patternMap, ok := rule.Validation.GetPattern().(map[string]interface{})
			if ok {
				return checkMetadata(patternMap)
			} else if rule.Validation.GetAnyPattern() != nil {
				anyPatterns, err := rule.Validation.DeserializeAnyPattern()
				if err != nil {
					logging.Error(err, "failed to deserialize anyPattern, expect type array")
					return false
				}

				for _, pattern := range anyPatterns {
					patternMap, ok := pattern.(map[string]interface{})
					if ok {
						ret := checkMetadata(patternMap)
						if !ret {
							return ret
						}
					}
				}
			}
		}
	}

	if rule.HasMutate() {
		if rule.Mutation.ForEachMutation != nil {
			for _, foreach := range rule.Mutation.ForEachMutation {
				forEachStrategicMergeMap, ok := foreach.GetPatchStrategicMerge().(map[string]interface{})
				if ok {
					return checkMetadata(forEachStrategicMergeMap)
				}
			}
		} else {
			strategicMergeMap, ok := rule.Mutation.GetPatchStrategicMerge().(map[string]interface{})
			if ok {
				return checkMetadata(strategicMergeMap)
			}
		}
	}

	return true
}

func ruleOnlyDealsWithResourceMetaData(rule kyvernov1.Rule) bool {
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
		logging.Error(err, "failed to deserialize anyPattern, expect type array")
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

func validateResources(path *field.Path, rule kyvernov1.Rule) (string, error) {
	// validate userInfo in match and exclude
	if errs := rule.ExcludeResources.UserInfo.Validate(path.Child("exclude")); len(errs) != 0 {
		return "exclude", errs.ToAggregate()
	}

	if (len(rule.MatchResources.Any) > 0 || len(rule.MatchResources.All) > 0) && !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyvernov1.ResourceDescription{}) {
		return "match.", fmt.Errorf("can't specify any/all together with match resources")
	}

	if (len(rule.ExcludeResources.Any) > 0 || len(rule.ExcludeResources.All) > 0) && !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyvernov1.ResourceDescription{}) {
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

	// validating the values present under validate.preconditions, if they exist
	if target := rule.GetAnyAllConditions(); target != nil {
		if path, err := validateConditions(target, "preconditions"); err != nil {
			return fmt.Sprintf("validate.%s", path), err
		}
	}
	// validating the values present under validate.conditions, if they exist
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
		return schemaKey, fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}

	// conditions are currently in the form of []interface{}
	kyvernoConditions, err := apiutils.ApiextensionsJsonToKyvernoConditions(conditions)
	if err != nil {
		return schemaKey, err
	}
	switch typedConditions := kyvernoConditions.(type) {
	case kyvernov1.AnyAllConditions:
		// validating the conditions under 'any', if there are any
		if !reflect.DeepEqual(typedConditions, kyvernov1.AnyAllConditions{}) && typedConditions.AnyConditions != nil {
			for i, condition := range typedConditions.AnyConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.any[%d].%s", schemaKey, i, path), err
				}
			}
		}
		// validating the conditions under 'all', if there are any
		if !reflect.DeepEqual(typedConditions, kyvernov1.AnyAllConditions{}) && typedConditions.AllConditions != nil {
			for i, condition := range typedConditions.AllConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.all[%d].%s", schemaKey, i, path), err
				}
			}
		}

	case []kyvernov1.Condition: // backwards compatibility
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
func validateConditionValues(c kyvernov1.Condition) (string, error) {
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

func validateValuesKeyRequest(c kyvernov1.Condition) (string, error) {
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
func validateConditionValuesKeyRequestOperation(c kyvernov1.Condition) (string, error) {
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

func validateRuleContext(rule kyvernov1.Rule) error {
	if rule.Context == nil || len(rule.Context) == 0 {
		return nil
	}

	for _, entry := range rule.Context {
		if entry.Name == "" {
			return fmt.Errorf("a name is required for context entries")
		}
		for _, v := range []string{"images", "request", "serviceAccountName", "serviceAccountNamespace", "element", "elementIndex"} {
			if entry.Name == v || strings.HasPrefix(entry.Name, v+".") {
				return fmt.Errorf("entry name %s is invalid as it conflicts with a pre-defined variable %s", entry.Name, v)
			}
		}

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

func validateVariable(entry kyvernov1.ContextEntry) error {
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

func validateConfigMap(entry kyvernov1.ContextEntry) error {
	if entry.ConfigMap.Name == "" {
		return fmt.Errorf("a name is required for configMap context entry")
	}

	if entry.ConfigMap.Namespace == "" {
		return fmt.Errorf("a namespace is required for configMap context entry")
	}

	return nil
}

func validateAPICall(entry kyvernov1.ContextEntry) error {
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

func validateImageRegistry(entry kyvernov1.ContextEntry) error {
	if entry.ImageRegistry.Reference == "" {
		return fmt.Errorf("a ref is required for imageRegistry context entry")
	}
	// Replace all variables to prevent validation failing on variable keys.
	ref := variables.ReplaceAllVars(entry.ImageRegistry.Reference, func(s string) string { return "kyvernoimageref" })

	// it's no use validating a reference that contains a variable
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
func validateMatchedResourceDescription(rd kyvernov1.ResourceDescription) (string, error) {
	if reflect.DeepEqual(rd, kyvernov1.ResourceDescription{}) {
		return "", fmt.Errorf("match resources not specified")
	}

	return "", nil
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

// jsonPatchOnPod checks if a rule applies JSON patches to Pod
func jsonPatchOnPod(rule kyvernov1.Rule) bool {
	if !rule.HasMutate() {
		return false
	}

	if slices.Contains(rule.MatchResources.Kinds, "Pod") && rule.Mutation.PatchesJSON6902 != "" {
		return true
	}

	return false
}

func podControllerAutoGenExclusion(policy kyvernov1.PolicyInterface) bool {
	annotations := policy.GetAnnotations()
	val, ok := annotations[kyvernov1.PodControllersAnnotation]
	if !ok || val == "none" {
		return false
	}

	reorderVal := strings.Split(strings.ToLower(val), ",")
	sort.Slice(reorderVal, func(i, j int) bool { return reorderVal[i] < reorderVal[j] })
	if ok && !reflect.DeepEqual(reorderVal, []string{"cronjob", "daemonset", "deployment", "job", "statefulset"}) {
		return true
	}
	return false
}

// validateWildcard check for an Match/Exclude block contains "*"
func validateWildcard(kinds []string, spec *kyvernov1.Spec, rule kyvernov1.Rule) error {
	if slices.Contains(kinds, "*") && spec.BackgroundProcessingEnabled() {
		return fmt.Errorf("wildcard policy not allowed in background mode. Set spec.background=false to disable background mode for this policy rule ")
	}
	if slices.Contains(kinds, "*") && len(kinds) > 1 {
		return fmt.Errorf("wildard policy can not deal more than one kind")
	}
	if slices.Contains(kinds, "*") {
		if rule.HasGenerate() || rule.HasVerifyImages() || rule.Validation.ForEachValidation != nil {
			return fmt.Errorf("wildcard policy does not support rule type")
		}

		if rule.HasValidate() {
			if rule.Validation.GetPattern() != nil || rule.Validation.GetAnyPattern() != nil {
				if !ruleOnlyDealsWithResourceMetaData(rule) {
					return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
						" the rule does not match any kind")
				}
			}

			if rule.Validation.Deny != nil {
				kyvernoConditions, _ := apiutils.ApiextensionsJsonToKyvernoConditions(rule.Validation.Deny.GetAnyAllConditions())
				switch typedConditions := kyvernoConditions.(type) {
				case []kyvernov1.Condition: // backwards compatibility
					for _, condition := range typedConditions {
						key := condition.GetKey()
						if !strings.Contains(key.(string), "request.object.metadata.") && (!wildCardAllowedVariables.MatchString(key.(string)) || strings.Contains(key.(string), "request.object.spec")) {
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
	}
	return nil
}

// validateKinds verifies if an API resource that matches 'kind' is valid kind
// and found in the cache, returns error if not found. It also returns an error if background scanning
// is enabled for a subresource.
func validateKinds(kinds []string, mock, backgroundScanningEnabled, isValidationPolicy bool, client dclient.Interface) error {
	for _, kind := range kinds {
		if !mock && !strings.Contains(kind, "*") {
			gv, k := kubeutils.GetKindFromGVK(kind)
			_, _, gvr, err := client.Discovery().FindResource(gv, k)
			if err != nil {
				return fmt.Errorf("unable to convert GVK to GVR for kinds %s, err: %s", k, err)
			}
			_, subresource := kubeutils.SplitSubresource(gvr.Resource)
			if subresource != "" && isValidationPolicy && backgroundScanningEnabled {
				return fmt.Errorf("background scan enabled with subresource %s", subresource)
			}
		}
	}
	return nil
}

func validateWildcardsWithNamespaces(enforce, audit, enforceW, auditW []string) error {
	pat, ns, notOk := wildcard.MatchPatterns(auditW, enforce...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' matches with namespace '%s'", pat, ns)
	}
	pat, ns, notOk = wildcard.MatchPatterns(enforceW, audit...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' matches with namespace '%s'", pat, ns)
	}

	pat1, pat2, notOk := wildcard.MatchPatterns(auditW, enforceW...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' conflicts with the pattern '%s'", pat1, pat2)
	}
	pat1, pat2, notOk = wildcard.MatchPatterns(enforceW, auditW...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' conflicts with the pattern '%s'", pat1, pat2)
	}

	return nil
}

func validateNamespaces(s *kyvernov1.Spec, path *field.Path) error {
	action := map[string]sets.Set[string]{
		"enforce":  sets.New[string](),
		"audit":    sets.New[string](),
		"enforceW": sets.New[string](),
		"auditW":   sets.New[string](),
	}

	for i, vfa := range s.ValidationFailureActionOverrides {
		patternList, nsList := wildcard.SeperateWildcards(vfa.Namespaces)

		if vfa.Action.Audit() {
			if action["enforce"].HasAny(nsList...) {
				return fmt.Errorf("conflicting namespaces found in path: %s: %s", path.Index(i).Child("namespaces").String(),
					strings.Join(sets.List(action["enforce"].Intersection(sets.New(nsList...))), ", "))
			}
			action["auditW"].Insert(patternList...)
		} else if vfa.Action.Enforce() {
			if action["audit"].HasAny(nsList...) {
				return fmt.Errorf("conflicting namespaces found in path: %s: %s", path.Index(i).Child("namespaces").String(),
					strings.Join(sets.List(action["audit"].Intersection(sets.New(nsList...))), ", "))
			}
			action["enforceW"].Insert(patternList...)
		}
		action[strings.ToLower(string(vfa.Action))].Insert(nsList...)

		err := validateWildcardsWithNamespaces(
			sets.List(action["enforce"]),
			sets.List(action["audit"]),
			sets.List(action["enforceW"]),
			sets.List(action["auditW"]),
		)
		if err != nil {
			return fmt.Errorf("path: %s: %s", path.Index(i).Child("namespaces").String(), err.Error())
		}
	}

	return nil
}

func checkForScaleSubresource(ruleTypeJson []byte, allKinds []string, warnings *[]string) {
	if strings.Contains(string(ruleTypeJson), "replicas") {
		for _, kind := range allKinds {
			if strings.Contains(strings.ToLower(kind), "scale") {
				return
			}
		}
		msg := "You are matching on replicas but not including the scale subresource in the policy."
		*warnings = append(*warnings, msg)
	}
}

func checkForStatusSubresource(ruleTypeJson []byte, allKinds []string, warnings *[]string) {
	if strings.Contains(string(ruleTypeJson), "status") {
		for _, kind := range allKinds {
			if strings.Contains(strings.ToLower(kind), "status") {
				return
			}
		}
		msg := "You are matching on status but not including the status subresource in the policy."
		*warnings = append(*warnings, msg)
	}
}
