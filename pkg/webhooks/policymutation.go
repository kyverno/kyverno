package webhooks

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ws *WebhookServer) handlePolicyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var policy *kyverno.ClusterPolicy
	raw := request.Object.Raw

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	if err := json.Unmarshal(raw, &policy); err != nil {
		glog.Errorf("Failed to unmarshal policy admission request, err %v\n", err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err),
			},
		}
	}
	// Generate JSON Patches for defaults
	patches, updateMsgs := generateJSONPatchesForDefaults(policy, request.Operation)
	if patches != nil {
		patchType := v1beta1.PatchTypeJSONPatch
		glog.V(4).Infof("defaulted values %v policy %s", updateMsgs, policy.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: strings.Join(updateMsgs, "'"),
			},
			Patch:     patches,
			PatchType: &patchType,
		}
	}
	glog.V(4).Infof("nothing to default for policy %s", policy.Name)
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func generateJSONPatchesForDefaults(policy *kyverno.ClusterPolicy, operation v1beta1.Operation) ([]byte, []string) {
	var patches [][]byte
	var updateMsgs []string

	// default 'ValidationFailureAction'
	if patch, updateMsg := defaultvalidationFailureAction(policy); patch != nil {
		patches = append(patches, patch)
		updateMsgs = append(updateMsgs, updateMsg)
	}

	// TODO(shuting): enable this feature on policy UPDATE
	if operation == v1beta1.Create {
		patch, errs := generatePodControllerRule(*policy)
		if len(errs) > 0 {
			var errMsgs []string
			for _, err := range errs {
				errMsgs = append(errMsgs, err.Error())
			}
			glog.Errorf("failed auto generatig rule for pod controllers: %s", errMsgs)
			updateMsgs = append(updateMsgs, strings.Join(errMsgs, ";"))
		}

		patches = append(patches, patch...)
	}
	return utils.JoinPatches(patches), updateMsgs
}

func defaultvalidationFailureAction(policy *kyverno.ClusterPolicy) ([]byte, string) {
	// default ValidationFailureAction to "audit" if not specified
	if policy.Spec.ValidationFailureAction == "" {
		glog.V(4).Infof("defaulting policy %s 'ValidationFailureAction' to '%s'", policy.Name, Audit)
		jsonPatch := struct {
			Path  string `json:"path"`
			Op    string `json:"op"`
			Value string `json:"value"`
		}{
			"/spec/validationFailureAction",
			"add",
			Audit, //audit
		}
		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			glog.Errorf("failed to set default 'ValidationFailureAction' to '%s' for policy %s", Audit, policy.Name)
			return nil, ""
		}
		glog.V(4).Infof("generate JSON Patch to set default 'ValidationFailureAction' to '%s' for policy %s", Audit, policy.Name)
		return patchByte, fmt.Sprintf("default 'ValidationFailureAction' to '%s'", Audit)
	}
	return nil, ""
}

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "null", user explicitely disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entrie match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod cotrollers

// generatePodControllerRule returns two patches: rulePatches and annotation patch(if necessary)
func generatePodControllerRule(policy kyverno.ClusterPolicy) (patches [][]byte, errs []error) {
	ann := policy.GetAnnotations()
	controllers, ok := ann[engine.PodControllersAnnotation]

	// scenario A
	if !ok {
		controllers = "all"
		annPatch, err := defaultPodControllerAnnotation(ann)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate pod controller annotation for policy '%s': %v", policy.Name, err))
		} else {
			patches = append(patches, annPatch)
		}
	}

	// scenario B
	if controllers == "null" {
		return nil, nil
	}

	glog.V(3).Infof("Auto generating rule for pod controller: %s", controllers)

	p, err := generateRulePatches(policy, controllers)
	patches = append(patches, p...)
	errs = append(errs, err...)
	return
}

// generateRulePatches generates rule for podControllers based on scenario A and C
func generateRulePatches(policy kyverno.ClusterPolicy, controllers string) (rulePatches [][]byte, errs []error) {
	var genRule kyvernoRule
	insertIdx := len(policy.Spec.Rules)

	for _, rule := range policy.Spec.Rules {
		genRule = generateRuleForControllers(rule, controllers)
		if reflect.DeepEqual(genRule, kyvernoRule{}) {
			continue
		}

		// generate patch bytes
		jsonPatch := struct {
			Path  string      `json:"path"`
			Op    string      `json:"op"`
			Value interface{} `json:"value"`
		}{
			fmt.Sprintf("/spec/rules/%s", strconv.Itoa(insertIdx)),
			"add",
			genRule,
		}
		pbytes, err := json.Marshal(jsonPatch)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// check the patch
		if _, err := jsonpatch.DecodePatch([]byte("[" + string(pbytes) + "]")); err != nil {
			errs = append(errs, err)
			continue
		}

		rulePatches = append(rulePatches, pbytes)
		insertIdx++
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

func generateRuleForControllers(rule kyverno.Rule, controllers string) kyvernoRule {
	match := rule.MatchResources
	exclude := rule.ExcludeResources
	if !utils.ContainsString(match.ResourceDescription.Kinds, "Pod") ||
		(len(exclude.ResourceDescription.Kinds) != 0 && !utils.ContainsString(exclude.ResourceDescription.Kinds, "Pod")) {
		return kyvernoRule{}
	}

	if rule.Mutation.Overlay == nil && !rule.HasValidate() {
		return kyvernoRule{}
	}

	// scenario A
	if controllers == "all" {
		if match.ResourceDescription.Name != "" || match.ResourceDescription.Selector != nil ||
			exclude.ResourceDescription.Name != "" || exclude.ResourceDescription.Selector != nil {
			glog.Warningf("Rule '%s' skip generating rule on pod controllers: Name / Selector in resource decription may not be applicable.", rule.Name)
			return kyvernoRule{}
		}
		controllers = engine.PodControllers
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

// defaultPodControllerAnnotation generates annotation "pod-policies.kyverno.io/autogen-controllers=all"
// ann passes in the annotation of the policy
func defaultPodControllerAnnotation(ann map[string]string) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string)
		ann[engine.PodControllersAnnotation] = "all"
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
		"all",
	}

	patchByte, err := json.Marshal(jsonPatch)
	if err != nil {
		return nil, err
	}
	return patchByte, nil
}
