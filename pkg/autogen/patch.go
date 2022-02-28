package autogen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "none", user explicitly disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entire match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod controllers

// generateRulePatches generates rule for podControllers based on scenario A and C
func GenerateRulePatches(spec *kyverno.Spec, controllers string, log logr.Logger) (rulePatches [][]byte, errs []error) {
	insertIdx := len(spec.Rules)

	ruleMap := createRuleMap(spec.Rules)
	var ruleIndex = make(map[string]int)
	for index, rule := range spec.Rules {
		ruleIndex[rule.Name] = index
	}

	for _, rule := range spec.Rules {
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

func updateGenRuleByte(pbyte []byte, kind string, genRule kyvernoRule) (obj []byte) {
	// TODO: do we need to unmarshall here ?
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
