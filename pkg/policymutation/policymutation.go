package policymutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/common"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/utils"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

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

	convertPatches, errs := convertOverlayToStrategicMerge(policy, log)

	if len(errs) > 0 {
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
			log.Error(err, "failed to generate pod controller rule")
		}
		updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
	}

	patches = append(patches, convertPatches...)

	return utils.JoinPatches(patches), updateMsgs
}

func convertOverlayToStrategicMerge(policy *kyverno.ClusterPolicy, log logr.Logger) ([][]byte, []error) {
	if len(policy.Spec.Rules) == 0 {
		errors.New("a policy should have at least one rule")
	}

	patches := make([][]byte, 0)

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
					log.Error(err, "failed to set default value", "spec.background", true)
				}

				patches = append(patches, patchByte)
			}
		}
	}

	return patches, nil
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

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "none", user explicitly disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entrie match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod cotrollers

// GeneratePodControllerRule returns two patches: rulePatches and annotation patch(if necessary)
func GeneratePodControllerRule(policy kyverno.ClusterPolicy, log logr.Logger) (patches [][]byte, errs []error) {
	ann := policy.GetAnnotations()
	controllers, ok := ann[engine.PodControllersAnnotation]

	// scenario A
	if !ok {
		controllers = engine.PodControllers
		annPatch, err := defaultPodControllerAnnotation(ann)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate pod controller annotation for policy '%s': %v", policy.Name, err))
		} else {
			patches = append(patches, annPatch)
		}
	}

	// scenario B
	if controllers == "none" {
		return nil, nil
	}

	log.V(3).Info("auto generating rule for pod controllers", "controllers", controllers)

	p, err := generateRulePatches(policy, controllers, log)
	patches = append(patches, p...)
	errs = append(errs, err...)
	return
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
			rulePatches = append(rulePatches, pbytes)
			insertIdx++
			patchPostion = insertIdx
		}

		// handle CronJob, it appends an additional rule
		genRule = generateCronJobRule(rule, controllers, log)
		if !reflect.DeepEqual(genRule, kyvernoRule{}) {
			pbytes := convertToPatches(genRule, patchPostion)
			rulePatches = append(rulePatches, pbytes)
			insertIdx++
		}
	}

	return
}

// the kyvernoRule holds the temporary kyverno rule struct
// each field is a pointer to the the actual object
// when serilizing data, we would expect to drop the omitempty key
// otherwise (without the pointer), it will be set to empty value
// - an empty struct in this case, some may fail the schema validation
// TODO(shuting) may related to:
// https://github.com/nirmata/kyverno/pull/549#discussion_r360088556
// https://github.com/nirmata/kyverno/issues/568

type kyvernoRule struct {
	Name             string                    `json:"name"`
	MatchResources   *kyverno.MatchResources   `json:"match"`
	ExcludeResources *kyverno.ExcludeResources `json:"exclude,omitempty"`
	Mutation         *kyverno.Mutation         `json:"mutate,omitempty"`
	Validation       *kyverno.Validation       `json:"validate,omitempty"`
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
	if !utils.ContainsString(match.ResourceDescription.Kinds, "Pod") ||
		(len(exclude.ResourceDescription.Kinds) != 0 && !utils.ContainsString(exclude.ResourceDescription.Kinds, "Pod")) {
		return kyvernoRule{}
	}

	if rule.Mutation.Overlay == nil && !rule.HasValidate() && rule.Mutation.PatchStrategicMerge == nil {
		return kyvernoRule{}
	}

	// Support backword compatibility
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
		if match.ResourceDescription.Name != "" || match.ResourceDescription.Selector != nil ||
			exclude.ResourceDescription.Name != "" || exclude.ResourceDescription.Selector != nil {
			logger.Info("skip generating rule on pod controllers: Name / Selector in resource decription may not be applicable.", "rule", rule.Name)
			return kyvernoRule{}
		}
		if controllers == "all" {
			controllers = "DaemonSet,Deployment,Job,StatefulSet"
		} else {
			controllers = strings.Join(controllersValidated, ",")
		}
	}

	controllerRule := &kyvernoRule{
		Name:           fmt.Sprintf("autogen-%s", rule.Name),
		MatchResources: match.DeepCopy(),
	}

	// overwrite Kinds by pod controllers defined in the annotation
	controllerRule.MatchResources.Kinds = strings.Split(controllers, ",")
	if len(exclude.Kinds) != 0 {
		controllerRule.ExcludeResources = exclude.DeepCopy()
		controllerRule.ExcludeResources.Kinds = strings.Split(controllers, ",")
	}

	if rule.Mutation.Overlay != nil {
		newMutation := &kyverno.Mutation{
			Overlay: map[string]interface{}{
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

	if rule.Validation.Pattern != nil {
		newValidate := &kyverno.Validation{
			Message: rule.Validation.Message,
			Pattern: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": rule.Validation.Pattern,
				},
			},
		}
		controllerRule.Validation = newValidate.DeepCopy()
		return *controllerRule
	}

	if len(rule.Validation.AnyPattern) != 0 {
		var patterns []interface{}
		for _, pattern := range rule.Validation.AnyPattern {
			newPattern := map[string]interface{}{
				"spec": map[string]interface{}{
					"template": pattern,
				},
			}

			patterns = append(patterns, newPattern)
		}

		controllerRule.Validation = &kyverno.Validation{
			Message:    rule.Validation.Message,
			AnyPattern: patterns,
		}
		return *controllerRule
	}

	return kyvernoRule{}
}

// defaultPodControllerAnnotation inserts an annotation
// "pod-policies.kyverno.io/autogen-controllers=DaemonSet,Deployment,Job,StatefulSet" to policy
func defaultPodControllerAnnotation(ann map[string]string) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string)
		ann[engine.PodControllersAnnotation] = engine.PodControllers
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
		engine.PodControllers,
	}

	patchByte, err := json.Marshal(jsonPatch)
	if err != nil {
		return nil, err
	}
	return patchByte, nil
}
