package common

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

// HasVariables - check for variables in the policy
func HasVariables(policy kyvernov1.PolicyInterface) [][]string {
	policyRaw, _ := json.Marshal(policy)
	matches := regex.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

func GetVariable(
	variablesString []string,
	vals *api.Values,
	valuesFile string,
	fs billy.Filesystem,
	policyResourcePath string,
) (map[string]string, map[string]string, map[string]map[string]api.Resource, map[string]map[string]string, []api.Subresource, error) {
	if vals == nil && valuesFile != "" {
		v, err := values.Load(fs, filepath.Join(policyResourcePath, valuesFile))
		if err != nil {
			fmt.Printf("Unable to load variable file: %s. error: %s \n", valuesFile, err)
			return nil, nil, nil, nil, nil, sanitizederror.NewWithError("unable to read yaml", err)
		}
		vals = v
	}

	variables, globalValMap, valuesMapResource, valuesMapRule, namespaceSelectorMap, subresources := getVariable(variablesString, vals)

	if globalValMap != nil {
		if _, ok := globalValMap["request.operation"]; !ok {
			globalValMap["request.operation"] = "CREATE"
			log.V(3).Info("Defaulting request.operation to CREATE")
		}
	}

	storePolicies := make([]store.Policy, 0)
	for policyName, ruleMap := range valuesMapRule {
		storeRules := make([]store.Rule, 0)
		for _, rule := range ruleMap {
			storeRules = append(storeRules, store.Rule{
				Name:          rule.Name,
				Values:        rule.Values,
				ForEachValues: rule.ForeachValues,
			})
		}
		storePolicies = append(storePolicies, store.Policy{
			Name:  policyName,
			Rules: storeRules,
		})
	}

	store.SetPolicies(storePolicies...)

	return variables, globalValMap, valuesMapResource, namespaceSelectorMap, subresources, nil
}

func getVariable(
	variablesString []string,
	vals *api.Values,
) (map[string]string, map[string]string, map[string]map[string]api.Resource, map[string]map[string]api.Rule, map[string]map[string]string, []api.Subresource) {
	valuesMapResource := make(map[string]map[string]api.Resource)
	valuesMapRule := make(map[string]map[string]api.Rule)
	namespaceSelectorMap := make(map[string]map[string]string)
	variables := make(map[string]string)
	subresources := make([]api.Subresource, 0)
	globalValMap := make(map[string]string)
	reqObjVars := ""
	for _, kvpair := range variablesString {
		kvs := strings.Split(strings.Trim(kvpair, " "), "=")
		if strings.Contains(kvs[0], "request.object") {
			if !strings.Contains(reqObjVars, kvs[0]) {
				reqObjVars = reqObjVars + "," + kvs[0]
			}
			continue
		}
		variables[strings.Trim(kvs[0], " ")] = strings.Trim(kvs[1], " ")
	}

	if vals != nil {
		if vals.GlobalValues == nil {
			vals.GlobalValues = make(map[string]string)
			vals.GlobalValues["request.operation"] = "CREATE"
			log.V(3).Info("Defaulting request.operation to CREATE")
		} else {
			if val, ok := vals.GlobalValues["request.operation"]; ok {
				if val == "" {
					vals.GlobalValues["request.operation"] = "CREATE"
					log.V(3).Info("Globally request.operation value provided by the user is empty, defaulting it to CREATE", "request.opearation: ", vals.GlobalValues)
				}
			}
		}

		globalValMap = vals.GlobalValues

		for _, p := range vals.Policies {
			resourceMap := make(map[string]api.Resource)
			for _, r := range p.Resources {
				if val, ok := r.Values["request.operation"]; ok {
					if val == "" {
						r.Values["request.operation"] = "CREATE"
						log.V(3).Info("No request.operation found, defaulting it to CREATE", "policy", p.Name)
					}
				}
				for variableInFile := range r.Values {
					if strings.Contains(variableInFile, "request.object") {
						if !strings.Contains(reqObjVars, variableInFile) {
							reqObjVars = reqObjVars + "," + variableInFile
						}
						delete(r.Values, variableInFile)
						continue
					}
				}
				resourceMap[r.Name] = r
			}
			valuesMapResource[p.Name] = resourceMap

			if p.Rules != nil {
				ruleMap := make(map[string]api.Rule)
				for _, r := range p.Rules {
					ruleMap[r.Name] = r
				}
				valuesMapRule[p.Name] = ruleMap
			}
		}

		for _, n := range vals.NamespaceSelectors {
			namespaceSelectorMap[n.Name] = n.Labels
		}

		subresources = vals.Subresources
	}
	if reqObjVars != "" {
		fmt.Printf("\nNOTICE: request.object.* variables are automatically parsed from the supplied resource. Ignoring value of variables `%v`.\n", reqObjVars)
	}
	return variables, globalValMap, valuesMapResource, valuesMapRule, namespaceSelectorMap, subresources
}

func SetInStoreContext(mutatedPolicies []kyvernov1.PolicyInterface, variables map[string]string) map[string]string {
	storePolicies := make([]store.Policy, 0)
	for _, policy := range mutatedPolicies {
		storeRules := make([]store.Rule, 0)
		for _, rule := range autogen.ComputeRules(policy) {
			contextVal := make(map[string]interface{})
			if len(rule.Context) != 0 {
				for _, contextVar := range rule.Context {
					for k, v := range variables {
						if strings.HasPrefix(k, contextVar.Name) {
							contextVal[k] = v
							delete(variables, k)
						}
					}
				}
				storeRules = append(storeRules, store.Rule{
					Name:   rule.Name,
					Values: contextVal,
				})
			}
		}
		storePolicies = append(storePolicies, store.Policy{
			Name:  policy.GetName(),
			Rules: storeRules,
		})
	}

	store.SetPolicies(storePolicies...)

	return variables
}

func CheckVariableForPolicy(valuesMap map[string]map[string]api.Resource, globalValMap map[string]string, policyName string, resourceName string, resourceKind string, variables map[string]string, kindOnwhichPolicyIsApplied map[string]struct{}, variable string) (map[string]interface{}, error) {
	// get values from file for this policy resource combination
	thisPolicyResourceValues := make(map[string]interface{})
	if len(valuesMap[policyName]) != 0 && !datautils.DeepEqual(valuesMap[policyName][resourceName], api.Resource{}) {
		thisPolicyResourceValues = valuesMap[policyName][resourceName].Values
	}

	for k, v := range variables {
		thisPolicyResourceValues[k] = v
	}

	if thisPolicyResourceValues == nil && len(globalValMap) > 0 {
		thisPolicyResourceValues = make(map[string]interface{})
	}

	for k, v := range globalValMap {
		if _, ok := thisPolicyResourceValues[k]; !ok {
			thisPolicyResourceValues[k] = v
		}
	}

	// skipping the variable check for non matching kind
	if _, ok := kindOnwhichPolicyIsApplied[resourceKind]; ok {
		if len(variable) > 0 && len(thisPolicyResourceValues) == 0 && store.HasPolicies() {
			return thisPolicyResourceValues, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policyName, resourceName), nil)
		}
	}
	return thisPolicyResourceValues, nil
}
