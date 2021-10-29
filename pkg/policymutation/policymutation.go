package policymutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// GenerateJSONPatchesForDefaults generates default JSON patches for
// - ValidationFailureAction
// - Background
// - auto-gen annotation and rules
func GenerateJSONPatchesForDefaults(policy *kyverno.ClusterPolicy, log logr.Logger) ([]byte, []string) {
	var patches [][]byte
	var updateMsgs []string

	// default 'ValidationFailureAction'
	if patch, updateMsg := defaultvalidationFailureAction(policy, log); patch != nil {
		patches = append(patches, patch)
		updateMsgs = append(updateMsgs, updateMsg)
	}

	// default 'Background'
	if patch, updateMsg := defaultBackgroundFlag(policy, log); patch != nil {
		patches = append(patches, patch)
		updateMsgs = append(updateMsgs, updateMsg)
	}

	if patch, updateMsg := defaultFailurePolicy(policy, log); patch != nil {
		patches = append(patches, patch)
		updateMsgs = append(updateMsgs, updateMsg)
	}

	patch, errs := GeneratePodControllerRule(*policy, log)
	if len(errs) > 0 {
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
			log.Error(err, "failed to generate pod controller rule")
		}
		updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
	}
	patches = append(patches, patch...)

	convertPatch, errs := convertPatchToJSON6902(policy, log)
	if len(errs) > 0 {
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
			log.Error(err, "failed to generate pod controller rule")
		}
		updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
	}
	patches = append(patches, convertPatch...)

	formatedGVK, errs := checkForGVKFormatPatch(policy, log)
	if len(errs) > 0 {
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
			log.Error(err, "failed to format the kind")
		}
		updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
	}
	patches = append(patches, formatedGVK...)

	overlaySMPPatches, errs := convertOverlayToStrategicMerge(policy, log)
	if len(errs) > 0 {
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
			log.Error(err, "failed to generate pod controller rule")
		}
		updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
	}
	patches = append(patches, overlaySMPPatches...)

	return utils.JoinPatches(patches), updateMsgs
}

func checkForGVKFormatPatch(policy *kyverno.ClusterPolicy, log logr.Logger) (patches [][]byte, errs []error) {
	patches = make([][]byte, 0)
	for i, rule := range policy.Spec.Rules {
		patchByte, err := convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/match/resources/kinds", strconv.Itoa(i)), rule.MatchResources.Kinds, log)
		if err == nil && patchByte != nil {
			patches = append(patches, patchByte)
		} else if err != nil {
			errs = append(errs, fmt.Errorf("failed to GVK for rule '%s/%s/%d/match': %v", policy.Name, rule.Name, i, err))
		}

		for j, matchAll := range rule.MatchResources.All {
			patchByte, err := convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/match/all/%s/resources/kinds", strconv.Itoa(i), strconv.Itoa(j)), matchAll.ResourceDescription.Kinds, log)
			if err == nil && patchByte != nil {
				patches = append(patches, patchByte)
			} else if err != nil {
				errs = append(errs, fmt.Errorf("failed to convert GVK for rule '%s/%s/%d/match/all/%d': %v", policy.Name, rule.Name, i, j, err))
			}
		}

		for k, matchAny := range rule.MatchResources.Any {
			patchByte, err := convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/match/any/%s/resources/kinds", strconv.Itoa(i), strconv.Itoa(k)), matchAny.ResourceDescription.Kinds, log)
			if err == nil && patchByte != nil {
				patches = append(patches, patchByte)
			} else if err != nil {
				errs = append(errs, fmt.Errorf("failed to convert GVK for rule '%s/%s/%d/match/any/%d': %v", policy.Name, rule.Name, i, k, err))
			}
		}

		patchByte, err = convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/exclude/resources/kinds", strconv.Itoa(i)), rule.ExcludeResources.Kinds, log)
		if err == nil && patchByte != nil {
			patches = append(patches, patchByte)
		} else if err != nil {
			errs = append(errs, fmt.Errorf("failed to convert GVK for rule '%s/%s/%d/exclude': %v", policy.Name, rule.Name, i, err))
		}

		for j, excludeAll := range rule.ExcludeResources.All {
			patchByte, err := convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/exclude/all/%s/resources/kinds", strconv.Itoa(i), strconv.Itoa(j)), excludeAll.ResourceDescription.Kinds, log)
			if err == nil && patchByte != nil {
				patches = append(patches, patchByte)
			} else if err != nil {
				errs = append(errs, fmt.Errorf("failed to convert GVK for rule '%s/%s/%d/exclude/all/%d': %v", policy.Name, rule.Name, i, j, err))
			}
		}

		for k, excludeAny := range rule.ExcludeResources.Any {
			patchByte, err := convertGVKForKinds(fmt.Sprintf("/spec/rules/%s/exclude/any/%s/resources/kinds", strconv.Itoa(i), strconv.Itoa(k)), excludeAny.ResourceDescription.Kinds, log)
			if err == nil && patchByte != nil {
				patches = append(patches, patchByte)
			} else if err != nil {
				errs = append(errs, fmt.Errorf("failed to convert GVK for rule '%s/%s/%d/exclude/any/%d': %v", policy.Name, rule.Name, i, k, err))
			}
		}
	}

	return patches, errs
}

func convertGVKForKinds(path string, kinds []string, log logr.Logger) ([]byte, error) {
	kindList := []string{}
	for _, k := range kinds {
		gvk := common.GetFormatedKind(k)
		if gvk == k {
			continue
		}
		kindList = append(kindList, gvk)
	}

	if len(kindList) == 0 {
		return nil, nil
	}

	p, err := buildReplaceJsonPatch(path, kindList)
	log.V(4).WithName("convertGVKForKinds").Info("generated patch", "patch", string(p))
	return p, err
}

func buildReplaceJsonPatch(path string, kindList []string) ([]byte, error) {
	jsonPatch := struct {
		Path  string   `json:"path"`
		Op    string   `json:"op"`
		Value []string `json:"value"`
	}{
		path,
		"replace",
		kindList,
	}
	return json.Marshal(jsonPatch)
}

func convertPatchToJSON6902(policy *kyverno.ClusterPolicy, log logr.Logger) (patches [][]byte, errs []error) {
	patches = make([][]byte, 0)

	for i, rule := range policy.Spec.Rules {
		if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
			if len(rule.Mutation.Patches) > 0 {
				mutation := rule.Mutation
				patchesJSON6902 := ""
				for _, patch := range mutation.Patches {
					patchesJSON6902 += fmt.Sprintf("- path : %s\n  op : %s\n  value: %s\n", patch.Path, patch.Operation, patch.Value)
				}
				mutation.PatchesJSON6902 = patchesJSON6902
				mutation.Patches = []kyverno.Patch{}

				jsonPatch := struct {
					Path  string            `json:"path"`
					Op    string            `json:"op"`
					Value *kyverno.Mutation `json:"value"`
				}{
					fmt.Sprintf("/spec/rules/%s/mutate", strconv.Itoa(i)),
					"replace",
					&mutation,
				}

				patchByte, err := json.Marshal(jsonPatch)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to convert patch to patchesJson6902 for policy '%s': %v", policy.Name, err))
				}

				patches = append(patches, patchByte)
			}
		}
	}

	return patches, errs
}

func convertOverlayToStrategicMerge(policy *kyverno.ClusterPolicy, log logr.Logger) (patches [][]byte, errs []error) {
	patches = make([][]byte, 0)
	if len(policy.Spec.Rules) == 0 {
		return patches, []error{
			errors.New("a policy should have at least one rule"),
		}
	}

	for i, rule := range policy.Spec.Rules {
		if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
			if !reflect.DeepEqual(rule.Mutation.Overlay, kyverno.Mutation{}.Overlay) {
				mutation := rule.Mutation
				mutation.PatchStrategicMerge = mutation.Overlay
				var a interface{}
				mutation.Overlay = a

				jsonPatch := struct {
					Path  string            `json:"path"`
					Op    string            `json:"op"`
					Value *kyverno.Mutation `json:"value"`
				}{
					fmt.Sprintf("/spec/rules/%s/mutate", strconv.Itoa(i)),
					"replace",
					&mutation,
				}

				patchByte, err := json.Marshal(jsonPatch)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to convert overlay to patchStrategicMerge for policy '%s': %v", policy.Name, err))
				}

				patches = append(patches, patchByte)
			}
		}
	}

	return patches, errs
}

func defaultBackgroundFlag(policy *kyverno.ClusterPolicy, log logr.Logger) ([]byte, string) {
	// set 'Background' flag to 'true' if not specified
	defaultVal := true
	if policy.Spec.Background == nil {
		log.V(4).Info("setting default value", "spec.background", true)
		jsonPatch := struct {
			Path  string `json:"path"`
			Op    string `json:"op"`
			Value *bool  `json:"value"`
		}{
			"/spec/background",
			"add",
			&defaultVal,
		}

		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			log.Error(err, "failed to set default value", "spec.background", true)
			return nil, ""
		}

		log.V(3).Info("generated JSON Patch to set default", "spec.background", true)
		return patchByte, fmt.Sprintf("default 'Background' to '%s'", strconv.FormatBool(true))
	}

	return nil, ""
}

func defaultvalidationFailureAction(policy *kyverno.ClusterPolicy, log logr.Logger) ([]byte, string) {
	// set ValidationFailureAction to "audit" if not specified
	Audit := common.Audit
	if policy.Spec.ValidationFailureAction == "" {
		log.V(4).Info("setting default value", "spec.validationFailureAction", Audit)

		jsonPatch := struct {
			Path  string `json:"path"`
			Op    string `json:"op"`
			Value string `json:"value"`
		}{
			"/spec/validationFailureAction",
			"add",
			Audit,
		}

		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			log.Error(err, "failed to default value", "spec.validationFailureAction", Audit)
			return nil, ""
		}

		log.V(3).Info("generated JSON Patch to set default", "spec.validationFailureAction", Audit)

		return patchByte, fmt.Sprintf("default 'ValidationFailureAction' to '%s'", Audit)
	}

	return nil, ""
}
func defaultFailurePolicy(policy *kyverno.ClusterPolicy, log logr.Logger) ([]byte, string) {
	// set failurePolicy to Fail if not present
	failurePolicy := string(kyverno.Fail)
	if policy.Spec.FailurePolicy == nil {
		log.V(4).Info("setting default value", "spec.failurePolicy", failurePolicy)
		jsonPatch := struct {
			Path  string `json:"path"`
			Op    string `json:"op"`
			Value string `json:"value"`
		}{
			"/spec/failurePolicy",
			"add",
			string(kyverno.Fail),
		}

		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			log.Error(err, "failed to set default value", "spec.failurePolicy", failurePolicy)
			return nil, ""
		}

		log.V(3).Info("generated JSON Patch to set default", "spec.failurePolicy", failurePolicy)
		return patchByte, fmt.Sprintf("default failurePolicy to '%s'", failurePolicy)
	}

	return nil, ""
}

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "none", user explicitly disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entire match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod controllers

// GeneratePodControllerRule returns two patches: rulePatches and annotation patch(if necessary)
func GeneratePodControllerRule(policy kyverno.ClusterPolicy, log logr.Logger) (patches [][]byte, errs []error) {
	applyAutoGen, desiredControllers := CanAutoGen(&policy, log)

	ann := policy.GetAnnotations()
	actualControllers, ok := ann[engine.PodControllersAnnotation]

	// - scenario A
	// - predefined controllers are invalid, overwrite the value
	if !ok || !applyAutoGen {
		actualControllers = desiredControllers
		annPatch, err := defaultPodControllerAnnotation(ann, actualControllers)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate pod controller annotation for policy '%s': %v", policy.Name, err))
		} else {
			patches = append(patches, annPatch)
		}
	} else {
		if !applyAutoGen {
			actualControllers = desiredControllers
		}
	}

	// scenario B
	if actualControllers == "none" {
		return patches, nil
	}

	log.V(3).Info("auto generating rule for pod controllers", "controllers", actualControllers)

	p, err := generateRulePatches(policy, actualControllers, log)
	patches = append(patches, p...)
	errs = append(errs, err...)
	return
}

// CanAutoGen checks whether the rule(s) (in policy) can be applied to Pod controllers
// returns controllers as:
// - "none" if:
//          - name or selector is defined
//          - mixed kinds (Pod + pod controller) is defined
//          - mutate.Patches/mutate.PatchesJSON6902/validate.deny/generate rule is defined
// - otherwise it returns all pod controllers
func CanAutoGen(policy *kyverno.ClusterPolicy, log logr.Logger) (applyAutoGen bool, controllers string) {
	for _, rule := range policy.Spec.Rules {
		match := rule.MatchResources
		exclude := rule.ExcludeResources

		if match.ResourceDescription.Name != "" || match.ResourceDescription.Selector != nil || match.ResourceDescription.Annotations != nil ||
			exclude.ResourceDescription.Name != "" || exclude.ResourceDescription.Selector != nil || exclude.ResourceDescription.Annotations != nil {
			log.V(3).Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, "none"
		}

		if isKindOtherthanPod(match.Kinds) || isKindOtherthanPod(exclude.Kinds) {
			return false, "none"
		}

		for _, value := range match.Any {
			if isKindOtherthanPod(value.Kinds) {
				return false, "none"
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match any block is not be applicable.", "rule", rule.Name)
				return false, "none"
			}
		}
		for _, value := range match.All {

			if isKindOtherthanPod(value.Kinds) {
				return false, "none"
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match all block is not be applicable.", "rule", rule.Name)
				return false, "none"
			}
		}
		for _, value := range exclude.Any {
			if isKindOtherthanPod(value.Kinds) {
				return false, "none"
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not be applicable.", "rule", rule.Name)
				return false, "none"
			}
		}
		for _, value := range exclude.All {

			if isKindOtherthanPod(value.Kinds) {
				return false, "none"
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not be applicable.", "rule", rule.Name)
				return false, "none"
			}
		}

		if rule.Mutation.Patches != nil || rule.Mutation.PatchesJSON6902 != "" ||
			rule.Validation.Deny != nil || rule.HasGenerate() {
			return false, "none"
		}
	}

	return true, engine.PodControllers
}

func isKindOtherthanPod(kinds []string) bool {
	if len(kinds) > 1 && utils.ContainsPod(kinds, "Pod") {
		return true
	}
	return false
}

func createRuleMap(rules []kyverno.Rule) map[string]kyvernoRule {
	var ruleMap = make(map[string]kyvernoRule)
	for _, rule := range rules {
		var jsonFriendlyStruct kyvernoRule

		jsonFriendlyStruct.Name = rule.Name

		if !reflect.DeepEqual(rule.MatchResources, kyverno.MatchResources{}) {
			jsonFriendlyStruct.MatchResources = rule.MatchResources.DeepCopy()
		}

		if !reflect.DeepEqual(rule.ExcludeResources, kyverno.ExcludeResources{}) {
			jsonFriendlyStruct.ExcludeResources = rule.ExcludeResources.DeepCopy()
		}

		if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
			jsonFriendlyStruct.Mutation = rule.Mutation.DeepCopy()
		}

		if !reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
			jsonFriendlyStruct.Validation = rule.Validation.DeepCopy()
		}

		ruleMap[rule.Name] = jsonFriendlyStruct
	}
	return ruleMap
}
func updateGenRuleByte(pbyte []byte, kind string, genRule kyvernoRule) (obj []byte) {
	if err := json.Unmarshal(pbyte, &genRule); err != nil {
		return obj
	}
	if kind == "Pod" {
		obj = []byte(strings.Replace(string(pbyte), "request.object.spec", "request.object.spec.template.spec", -1))
	}
	if kind == "Cronjob" {
		obj = []byte(strings.Replace(string(pbyte), "request.object.spec", "request.object.spec.jobTemplate.spec.template.spec", -1))
	}
	obj = []byte(strings.Replace(string(obj), "request.object.metadata", "request.object.spec.template.metadata", -1))
	return obj
}

// generateRulePatches generates rule for podControllers based on scenario A and C
func generateRulePatches(policy kyverno.ClusterPolicy, controllers string, log logr.Logger) (rulePatches [][]byte, errs []error) {
	insertIdx := len(policy.Spec.Rules)

	ruleMap := createRuleMap(policy.Spec.Rules)
	var ruleIndex = make(map[string]int)
	for index, rule := range policy.Spec.Rules {
		ruleIndex[rule.Name] = index
	}

	for _, rule := range policy.Spec.Rules {
		patchPostion := insertIdx
		convertToPatches := func(genRule kyvernoRule, patchPostion int) []byte {
			operation := "add"
			if existingAutoGenRule, alreadyExists := ruleMap[genRule.Name]; alreadyExists {
				existingAutoGenRuleRaw, _ := json.Marshal(existingAutoGenRule)
				genRuleRaw, _ := json.Marshal(genRule)

				if string(existingAutoGenRuleRaw) == string(genRuleRaw) {
					return nil
				}
				operation = "replace"
				patchPostion = ruleIndex[genRule.Name]
			}

			// generate patch bytes
			jsonPatch := struct {
				Path  string      `json:"path"`
				Op    string      `json:"op"`
				Value interface{} `json:"value"`
			}{
				fmt.Sprintf("/spec/rules/%s", strconv.Itoa(patchPostion)),
				operation,
				genRule,
			}
			pbytes, err := json.Marshal(jsonPatch)
			if err != nil {
				errs = append(errs, err)
				return nil
			}

			// check the patch
			if _, err := jsonpatch.DecodePatch([]byte("[" + string(pbytes) + "]")); err != nil {
				errs = append(errs, err)
				return nil
			}

			return pbytes
		}

		// handle all other controllers other than CronJob
		genRule := generateRuleForControllers(rule, stripCronJob(controllers), log)
		if !reflect.DeepEqual(genRule, kyvernoRule{}) {
			pbytes := convertToPatches(genRule, patchPostion)
			pbytes = updateGenRuleByte(pbytes, "Pod", genRule)
			if pbytes != nil {
				rulePatches = append(rulePatches, pbytes)
			}
			insertIdx++
			patchPostion = insertIdx
		}

		// handle CronJob, it appends an additional rule
		genRule = generateCronJobRule(rule, controllers, log)

		if !reflect.DeepEqual(genRule, kyvernoRule{}) {
			pbytes := convertToPatches(genRule, patchPostion)
			pbytes = updateGenRuleByte(pbytes, "Cronjob", genRule)
			if pbytes != nil {
				rulePatches = append(rulePatches, pbytes)
			}
			insertIdx++
		}
	}
	return
}

// the kyvernoRule holds the temporary kyverno rule struct
// each field is a pointer to the the actual object
// when serializing data, we would expect to drop the omitempty key
// otherwise (without the pointer), it will be set to empty value
// - an empty struct in this case, some may fail the schema validation
// may related to:
// https://github.com/kyverno/kyverno/pull/549#discussion_r360088556
// https://github.com/kyverno/kyverno/issues/568

type kyvernoRule struct {
	Name             string                       `json:"name"`
	MatchResources   *kyverno.MatchResources      `json:"match"`
	ExcludeResources *kyverno.ExcludeResources    `json:"exclude,omitempty"`
	Context          *[]kyverno.ContextEntry      `json:"context,omitempty"`
	AnyAllConditions *apiextensions.JSON          `json:"preconditions,omitempty"`
	Mutation         *kyverno.Mutation            `json:"mutate,omitempty"`
	Validation       *kyverno.Validation          `json:"validate,omitempty"`
	VerifyImages     []*kyverno.ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`
}

func generateRuleForControllers(rule kyverno.Rule, controllers string, log logr.Logger) kyvernoRule {
	logger := log.WithName("generateRuleForControllers")

	if strings.HasPrefix(rule.Name, "autogen-") || controllers == "" {
		logger.V(5).Info("skip generateRuleForControllers")
		return kyvernoRule{}
	}

	logger.V(3).Info("processing rule", "rulename", rule.Name)

	match := rule.MatchResources
	exclude := rule.ExcludeResources

	matchResourceDescriptionsKinds := rule.MatchKinds()
	excludeResourceDescriptionsKinds := rule.ExcludeKinds()

	if !utils.ContainsPod(matchResourceDescriptionsKinds, "Pod") ||
		(len(excludeResourceDescriptionsKinds) != 0 && !utils.ContainsPod(excludeResourceDescriptionsKinds, "Pod")) {
		return kyvernoRule{}
	}

	// Support backwards compatibility
	skipAutoGeneration := false
	var controllersValidated []string
	if controllers == "all" {
		skipAutoGeneration = true
	} else if controllers != "none" && controllers != "all" {
		controllersList := map[string]int{"DaemonSet": 1, "Deployment": 1, "Job": 1, "StatefulSet": 1}
		for _, value := range strings.Split(controllers, ",") {
			if _, ok := controllersList[value]; ok {
				controllersValidated = append(controllersValidated, value)
			}
		}
		if len(controllersValidated) > 0 {
			skipAutoGeneration = true
		}
	}

	if skipAutoGeneration {
		if controllers == "all" {
			controllers = "DaemonSet,Deployment,Job,StatefulSet"
		} else {
			controllers = strings.Join(controllersValidated, ",")
		}
	}

	name := fmt.Sprintf("autogen-%s", rule.Name)
	if len(name) > 63 {
		name = name[:63]
	}

	controllerRule := &kyvernoRule{
		Name:           name,
		MatchResources: match.DeepCopy(),
	}

	if len(rule.Context) > 0 {
		controllerRule.Context = &rule.DeepCopy().Context
	}

	kyvernoAnyAllConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.AnyAllConditions)
	switch typedAnyAllConditions := kyvernoAnyAllConditions.(type) {
	case kyverno.AnyAllConditions:
		if !reflect.DeepEqual(typedAnyAllConditions, kyverno.AnyAllConditions{}) {
			controllerRule.AnyAllConditions = &rule.DeepCopy().AnyAllConditions
		}
	case []kyverno.Condition:
		if len(typedAnyAllConditions) > 0 {
			controllerRule.AnyAllConditions = &rule.DeepCopy().AnyAllConditions
		}
	}

	if !reflect.DeepEqual(exclude, kyverno.ExcludeResources{}) {
		controllerRule.ExcludeResources = exclude.DeepCopy()
	}

	// overwrite Kinds by pod controllers defined in the annotation
	if len(rule.MatchResources.Any) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.MatchResources.Any, controllers)
		controllerRule.MatchResources.Any = rule
	} else if len(rule.MatchResources.All) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.MatchResources.All, controllers)
		controllerRule.MatchResources.All = rule
	} else {
		controllerRule.MatchResources.Kinds = strings.Split(controllers, ",")
	}

	if len(rule.ExcludeResources.Any) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.ExcludeResources.Any, controllers)
		controllerRule.ExcludeResources.Any = rule
	} else if len(rule.ExcludeResources.All) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.ExcludeResources.All, controllers)
		controllerRule.ExcludeResources.All = rule
	} else {
		if len(exclude.Kinds) != 0 {
			controllerRule.ExcludeResources.Kinds = strings.Split(controllers, ",")
		}
	}

	if rule.Mutation.Overlay != nil {
		newMutation := &kyverno.Mutation{
			PatchStrategicMerge: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": rule.Mutation.Overlay,
				},
			},
		}

		controllerRule.Mutation = newMutation.DeepCopy()
		return *controllerRule
	}

	if rule.Mutation.PatchStrategicMerge != nil {
		newMutation := &kyverno.Mutation{
			PatchStrategicMerge: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": rule.Mutation.PatchStrategicMerge,
				},
			},
		}

		controllerRule.Mutation = newMutation.DeepCopy()
		return *controllerRule
	}

	if len(rule.Mutation.ForEachMutation) > 0 && rule.Mutation.ForEachMutation != nil {
		var newForeachMutation []*kyverno.ForEachMutation
		for _, foreach := range rule.Mutation.ForEachMutation {
			newForeachMutation = append(newForeachMutation, &kyverno.ForEachMutation{
				List:             foreach.List,
				AnyAllConditions: foreach.AnyAllConditions,
				PatchStrategicMerge: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": foreach.PatchStrategicMerge,
					},
				},
			})
		}
		controllerRule.Mutation = &kyverno.Mutation{
			ForEachMutation: newForeachMutation,
		}
		return *controllerRule
	}

	if rule.Validation.Pattern != nil {
		newValidate := &kyverno.Validation{
			Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "pattern"),
			Pattern: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": rule.Validation.Pattern,
				},
			},
		}
		controllerRule.Validation = newValidate.DeepCopy()
		return *controllerRule
	}

	if rule.Validation.AnyPattern != nil {

		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			logger.Error(err, "failed to deserialize anyPattern, expect type array")
		}

		patterns := validateAnyPattern(anyPatterns)
		controllerRule.Validation = &kyverno.Validation{
			Message:    variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "anyPattern"),
			AnyPattern: patterns,
		}
		return *controllerRule
	}

	if len(rule.Validation.ForEachValidation) > 0 && rule.Validation.ForEachValidation != nil {
		newForeachValidate := make([]*kyverno.ForEachValidation, len(rule.Validation.ForEachValidation))
		for i, foreach := range rule.Validation.ForEachValidation {
			newForeachValidate[i] = foreach
		}
		controllerRule.Validation = &kyverno.Validation{
			Message:           variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "pattern"),
			ForEachValidation: newForeachValidate,
		}
		return *controllerRule
	}

	if rule.VerifyImages != nil {
		newVerifyImages := make([]*kyverno.ImageVerification, len(rule.VerifyImages))
		for i, vi := range rule.VerifyImages {
			newVerifyImages[i] = vi.DeepCopy()
		}

		controllerRule.VerifyImages = newVerifyImages
		return *controllerRule
	}

	return kyvernoRule{}
}

func validateAnyPattern(anyPatterns []interface{}) []interface{} {
	var patterns []interface{}
	for _, pattern := range anyPatterns {
		newPattern := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": pattern,
			},
		}

		patterns = append(patterns, newPattern)
	}
	return patterns
}

func getAnyAllAutogenRule(v kyverno.ResourceFilters, controllers string) kyverno.ResourceFilters {
	anyKind := v.DeepCopy()

	for i, value := range v {
		if utils.ContainsPod(value.Kinds, "Pod") {
			anyKind[i].Kinds = strings.Split(controllers, ",")
		}
	}
	return anyKind
}

// defaultPodControllerAnnotation inserts an annotation
// "pod-policies.kyverno.io/autogen-controllers=<controllers>" to policy
func defaultPodControllerAnnotation(ann map[string]string, controllers string) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string)
		ann[engine.PodControllersAnnotation] = controllers
		jsonPatch := struct {
			Path  string      `json:"path"`
			Op    string      `json:"op"`
			Value interface{} `json:"value"`
		}{
			"/metadata/annotations",
			"add",
			ann,
		}

		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			return nil, err
		}
		return patchByte, nil
	}

	jsonPatch := struct {
		Path  string      `json:"path"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}{
		"/metadata/annotations/pod-policies.kyverno.io~1autogen-controllers",
		"add",
		controllers,
	}

	patchByte, err := json.Marshal(jsonPatch)
	if err != nil {
		return nil, err
	}
	return patchByte, nil
}
