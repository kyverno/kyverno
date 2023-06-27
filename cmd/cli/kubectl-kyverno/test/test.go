package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"golang.org/x/exp/slices"
	"k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func applyPoliciesFromPath(
	fs billy.Filesystem,
	policyBytes []byte,
	isGit bool,
	policyResourcePath string,
	rc *resultCounts,
	openApiManager openapi.Manager,
	filter filter,
	auditWarn bool,
) (map[string]policyreportv1alpha2.PolicyReportResult, []api.TestResults, error) {
	engineResponses := make([]engineapi.EngineResponse, 0)
	var dClient dclient.Interface
	values := &api.Test{}
	var variablesString string
	var resultCounts common.ResultCounts

	store.SetLocal(true)
	if err := json.Unmarshal(policyBytes, values); err != nil {
		return nil, nil, sanitizederror.NewWithError("failed to decode yaml", err)
	}

	var filteredResults []api.TestResults
	for _, res := range values.Results {
		if filter(res) {
			filteredResults = append(filteredResults, res)
		}
	}
	values.Results = filteredResults

	if len(values.Results) == 0 {
		return nil, nil, nil
	}

	fmt.Printf("\nExecuting %s...\n", values.Name)
	valuesFile := values.Variables
	userInfoFile := values.UserInfo

	variables, globalValMap, valuesMap, namespaceSelectorMap, subresources, err := common.GetVariable(variablesString, values.Variables, fs, isGit, policyResourcePath)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			return nil, nil, sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return nil, nil, err
	}

	// get the user info as request info from a different file
	var userInfo v1beta1.RequestInfo

	if userInfoFile != "" {
		userInfo, err = common.GetUserInfoFromPath(fs, userInfoFile, isGit, policyResourcePath)
		if err != nil {
			fmt.Printf("Error: failed to load request info\nCause: %s\n", err)
			os.Exit(1)
		}
	}

	policyFullPath := getFullPath(values.Policies, policyResourcePath, isGit)
	resourceFullPath := getFullPath(values.Resources, policyResourcePath, isGit)

	for i, result := range values.Results {
		arrPatchedResource := []string{result.PatchedResource}
		arrGeneratedResource := []string{result.GeneratedResource}
		arrCloneSourceResource := []string{result.CloneSourceResource}

		patchedResourceFullPath := getFullPath(arrPatchedResource, policyResourcePath, isGit)
		generatedResourceFullPath := getFullPath(arrGeneratedResource, policyResourcePath, isGit)
		CloneSourceResourceFullPath := getFullPath(arrCloneSourceResource, policyResourcePath, isGit)

		values.Results[i].PatchedResource = patchedResourceFullPath[0]
		values.Results[i].GeneratedResource = generatedResourceFullPath[0]
		values.Results[i].CloneSourceResource = CloneSourceResourceFullPath[0]
	}

	policies, validatingAdmissionPolicies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	var filteredPolicies []kyvernov1.PolicyInterface
	for _, p := range policies {
		for _, res := range values.Results {
			if p.GetName() == res.Policy {
				filteredPolicies = append(filteredPolicies, p)
				break
			}
		}
	}

	var filteredVAPs []v1alpha1.ValidatingAdmissionPolicy
	for _, p := range validatingAdmissionPolicies {
		for _, res := range values.Results {
			if p.GetName() == res.Policy {
				filteredVAPs = append(filteredVAPs, p)
				break
			}
		}
	}
	validatingAdmissionPolicies = filteredVAPs

	ruleToCloneSourceResource := map[string]string{}
	for _, p := range filteredPolicies {
		var filteredRules []kyvernov1.Rule

		for _, rule := range autogen.ComputeRules(p) {
			for _, res := range values.Results {
				if res.IsVap {
					continue
				}

				if rule.Name == res.Rule {
					filteredRules = append(filteredRules, rule)
					if rule.HasGenerate() {
						ruleUnstr, err := generate.GetUnstrRule(rule.Generation.DeepCopy())
						if err != nil {
							fmt.Printf("Error: failed to get unstructured rule\nCause: %s\n", err)
							break
						}

						genClone, _, err := unstructured.NestedMap(ruleUnstr.Object, "clone")
						if err != nil {
							fmt.Printf("Error: failed to read data\nCause: %s\n", err)
							break
						}

						if len(genClone) != 0 {
							ruleToCloneSourceResource[rule.Name] = res.CloneSourceResource
						}
					}
					break
				}
			}
		}
		p.GetSpec().SetRules(filteredRules)
	}
	policies = filteredPolicies

	err = common.PrintMutatedPolicy(policies)
	if err != nil {
		return nil, nil, sanitizederror.NewWithError("failed to print mutated policy", err)
	}

	resources, err := common.GetResourceAccordingToResourcePath(fs, resourceFullPath, false, policies, validatingAdmissionPolicies, dClient, "", false, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		os.Exit(1)
	}

	checkableResources := selectResourcesForCheck(resources, values)

	msgPolicies := "1 policy"
	if len(policies)+len(validatingAdmissionPolicies) > 1 {
		msgPolicies = fmt.Sprintf("%d policies", len(policies)+len(validatingAdmissionPolicies))
	}

	msgResources := "1 resource"
	if len(checkableResources) > 1 {
		msgResources = fmt.Sprintf("%d resources", len(checkableResources))
	}

	if len(policies) > 0 && len(checkableResources) > 0 {
		fmt.Printf("applying %s to %s... \n", msgPolicies, msgResources)
	}

	for _, policy := range policies {
		_, err := policyvalidation.Validate(policy, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName()))
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", policy.GetName())
			continue
		}

		matches := common.HasVariables(policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)

		if len(variable) > 0 {
			if len(variables) == 0 {
				// check policy in variable file
				if valuesFile == "" || valuesMap[policy.GetName()] == nil {
					fmt.Printf("test skipped for policy  %v  (as required variables are not provided by the users) \n \n", policy.GetName())
				}
			}
		}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, subresources, dClient)

		for _, resource := range checkableResources {
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				return nil, nil, sanitizederror.NewWithError(fmt.Sprintf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag", policy.GetName(), resource.GetName()), err)
			}
			applyPolicyConfig := common.ApplyPolicyConfig{
				Policy:                    policy,
				Resource:                  resource,
				MutateLogPath:             "",
				Variables:                 thisPolicyResourceValues,
				UserInfo:                  userInfo,
				PolicyReport:              true,
				NamespaceSelectorMap:      namespaceSelectorMap,
				Rc:                        &resultCounts,
				RuleToCloneSourceResource: ruleToCloneSourceResource,
				Client:                    dClient,
				Subresources:              subresources,
			}
			ers, err := common.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				return nil, nil, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers...)
		}
	}

	validatingAdmissionPolicy := common.ValidatingAdmissionPolicies{}
	for _, policy := range validatingAdmissionPolicies {
		for _, resource := range resources {
			applyPolicyConfig := common.ApplyPolicyConfig{
				ValidatingAdmissionPolicy: policy,
				Resource:                  resource,
				PolicyReport:              true,
				Rc:                        &resultCounts,
				Client:                    dClient,
				Subresources:              subresources,
			}
			ers, err := validatingAdmissionPolicy.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				return nil, nil, sanitizederror.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName()).Error(), err)
			}
			engineResponses = append(engineResponses, ers...)
		}
	}
	resultsMap, testResults := buildPolicyResults(engineResponses, values.Results, policyResourcePath, fs, isGit, auditWarn)
	return resultsMap, testResults, nil
}

func getFullPath(paths []string, policyResourcePath string, isGit bool) []string {
	var pols []string
	var pol string
	if !isGit {
		for _, path := range paths {
			pol = filepath.Join(policyResourcePath, path)
			pols = append(pols, pol)
		}
		return pols
	}
	return paths
}

func selectResourcesForCheck(resources []*unstructured.Unstructured, values *api.Test) []*unstructured.Unstructured {
	res, _, _ := selectResourcesForCheckInternal(resources, values)
	return res
}

// selectResourcesForCheckInternal internal method to test duplicates and unused
func selectResourcesForCheckInternal(resources []*unstructured.Unstructured, values *api.Test) ([]*unstructured.Unstructured, int, int) {
	var duplicates int
	var unused int
	uniqResources := make(map[string]*unstructured.Unstructured)

	for i := range resources {
		r := resources[i]
		key := fmt.Sprintf("%s/%s/%s", r.GetKind(), r.GetName(), r.GetNamespace())
		if _, ok := uniqResources[key]; ok {
			fmt.Println("skipping duplicate resource, resource :", r)
			duplicates++
		} else {
			uniqResources[key] = r
		}
	}

	selectedResources := map[string]*unstructured.Unstructured{}
	for key := range uniqResources {
		r := uniqResources[key]
		for _, res := range values.Results {
			if res.Kind == r.GetKind() {
				for _, testr := range res.Resources {
					if r.GetName() == testr {
						selectedResources[key] = r
					}
				}
				if r.GetName() == res.Resource {
					selectedResources[key] = r
				}
			}
		}
	}

	var checkableResources []*unstructured.Unstructured

	for key := range selectedResources {
		checkableResources = append(checkableResources, selectedResources[key])
		delete(uniqResources, key)
	}
	for _, r := range uniqResources {
		fmt.Println("skipping unused resource, resource :", r)
		unused++
	}
	return checkableResources, duplicates, unused
}

func buildPolicyResults(
	engineResponses []engineapi.EngineResponse,
	testResults []api.TestResults,
	policyResourcePath string,
	fs billy.Filesystem,
	isGit bool,
	auditWarn bool,
) (map[string]policyreportv1alpha2.PolicyReportResult, []api.TestResults) {
	results := map[string]policyreportv1alpha2.PolicyReportResult{}

	for _, resp := range engineResponses {
		var ns, name string
		var ann map[string]string

		isVAP := resp.IsValidatingAdmissionPolicy()

		if isVAP {
			validatingAdmissionPolicy := resp.ValidatingAdmissionPolicy()
			ns = validatingAdmissionPolicy.GetNamespace()
			name = validatingAdmissionPolicy.GetName()
			ann = validatingAdmissionPolicy.GetAnnotations()
		} else {
			kyvernoPolicy := resp.Policy()
			ns = kyvernoPolicy.GetNamespace()
			name = kyvernoPolicy.GetName()
			ann = kyvernoPolicy.GetAnnotations()
		}

		policyName := name
		resourceName := resp.Resource.GetName()
		resourceKind := resp.Resource.GetKind()
		resourceNamespace := resp.Resource.GetNamespace()
		policyNamespace := ns

		var rules []string
		for _, rule := range resp.PolicyResponse.Rules {
			rules = append(rules, rule.Name())
		}

		result := policyreportv1alpha2.PolicyReportResult{
			Policy: policyName,
			Resources: []corev1.ObjectReference{
				{
					Name: resourceName,
				},
			},
			Message: buildMessage(resp),
		}

		var patchedResourcePath []string
		for i, test := range testResults {
			var userDefinedPolicyNamespace string
			var userDefinedPolicyName string
			found, err := isNamespacedPolicy(test.Policy)
			if err != nil {
				log.Log.V(3).Info("error while checking the policy is namespaced or not", "policy: ", test.Policy, "error: ", err)
				continue
			}

			if found {
				userDefinedPolicyNamespace, userDefinedPolicyName = getUserDefinedPolicyNameAndNamespace(test.Policy)
				test.Policy = userDefinedPolicyName
			}

			if test.Resources != nil {
				if test.Policy == policyName {
					// results[].namespace value implicit set same as metadata.namespace until and unless
					// user provides explicit values for results[].namespace in test yaml file.
					if test.Namespace == "" {
						test.Namespace = resourceNamespace
						testResults[i].Namespace = resourceNamespace
					}
					for _, resource := range test.Resources {
						if resource == resourceName {
							var resultsKey string
							resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource, test.IsVap)
							if !test.IsVap {
								if !slices.Contains(rules, test.Rule) {
									if !slices.Contains(rules, "autogen-"+test.Rule) {
										if !slices.Contains(rules, "autogen-cronjob-"+test.Rule) {
											result.Result = policyreportv1alpha2.StatusSkip
										} else {
											testResults[i].AutoGeneratedRule = "autogen-cronjob"
											test.Rule = "autogen-cronjob-" + test.Rule
											resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource, test.IsVap)
										}
									} else {
										testResults[i].AutoGeneratedRule = "autogen"
										test.Rule = "autogen-" + test.Rule
										resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, resource, test.IsVap)
									}

									if results[resultsKey].Result == "" {
										result.Result = policyreportv1alpha2.StatusSkip
										results[resultsKey] = result
									}
								}

								patchedResourcePath = append(patchedResourcePath, test.PatchedResource)
							}

							if _, ok := results[resultsKey]; !ok {
								results[resultsKey] = result
							}
						}
					}
				}
			}
			if test.Resource != "" {
				if test.Policy == policyName && test.Resource == resourceName {
					var resultsKey string
					resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource, test.IsVap)
					if !test.IsVap {
						if !slices.Contains(rules, test.Rule) {
							if !slices.Contains(rules, "autogen-"+test.Rule) {
								if !slices.Contains(rules, "autogen-cronjob-"+test.Rule) {
									result.Result = policyreportv1alpha2.StatusSkip
								} else {
									testResults[i].AutoGeneratedRule = "autogen-cronjob"
									test.Rule = "autogen-cronjob-" + test.Rule
									resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource, test.IsVap)
								}
							} else {
								testResults[i].AutoGeneratedRule = "autogen"
								test.Rule = "autogen-" + test.Rule
								resultsKey = GetResultKeyAccordingToTestResults(userDefinedPolicyNamespace, test.Policy, test.Rule, test.Namespace, test.Kind, test.Resource, test.IsVap)
							}

							if results[resultsKey].Result == "" {
								result.Result = policyreportv1alpha2.StatusSkip
								results[resultsKey] = result
							}
						}

						patchedResourcePath = append(patchedResourcePath, test.PatchedResource)
					}

					if _, ok := results[resultsKey]; !ok {
						results[resultsKey] = result
					}
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Generation || test.Rule != rule.Name() {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName, test.IsVap)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else {
						var x string
						result.Result = policyreportv1alpha2.StatusFail
						x = getAndCompareResource(test.GeneratedResource, rule.GeneratedResource(), isGit, policyResourcePath, fs, true)
						if x == "pass" {
							result.Result = policyreportv1alpha2.StatusPass
						}
					}
					results[resultKey] = result
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Mutation || test.Rule != rule.Name() {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName, test.IsVap)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else {
						var x string
						for _, path := range patchedResourcePath {
							result.Result = policyreportv1alpha2.StatusFail
							x = getAndCompareResource(path, resp.PatchedResource, isGit, policyResourcePath, fs, false)
							if x == "pass" {
								result.Result = policyreportv1alpha2.StatusPass
								break
							}
						}
					}

					results[resultKey] = result
				}
			}

			for _, rule := range resp.PolicyResponse.Rules {
				if rule.RuleType() != engineapi.Validation && rule.RuleType() != engineapi.ImageVerify || test.Rule != rule.Name() && !test.IsVap {
					continue
				}

				var resultsKey []string
				var resultKey string
				var result policyreportv1alpha2.PolicyReportResult
				resultsKey = GetAllPossibleResultsKey(policyNamespace, policyName, rule.Name(), resourceNamespace, resourceKind, resourceName, test.IsVap)
				for _, key := range resultsKey {
					if val, ok := results[key]; ok {
						result = val
						resultKey = key
					} else {
						continue
					}

					if rule.Status() == engineapi.RuleStatusSkip {
						result.Result = policyreportv1alpha2.StatusSkip
					} else if rule.Status() == engineapi.RuleStatusError {
						result.Result = policyreportv1alpha2.StatusError
					} else if rule.Status() == engineapi.RuleStatusPass {
						result.Result = policyreportv1alpha2.StatusPass
					} else if rule.Status() == engineapi.RuleStatusFail {
						if scored, ok := ann[kyvernov1.AnnotationPolicyScored]; ok && scored == "false" {
							result.Result = policyreportv1alpha2.StatusWarn
						} else if auditWarn && resp.GetValidationFailureAction().Audit() {
							result.Result = policyreportv1alpha2.StatusWarn
						} else {
							result.Result = policyreportv1alpha2.StatusFail
						}
					} else {
						fmt.Println(rule)
					}

					results[resultKey] = result
				}
			}
		}
	}
	return results, testResults
}

func GetAllPossibleResultsKey(policyNamespace, policy, rule, resourceNamespace, kind, resource string, isVap bool) []string {
	var resultsKey []string
	var resultKey1, resultKey2, resultKey3, resultKey4 string

	if isVap {
		resultKey1 = fmt.Sprintf("%s-%s-%s", policy, kind, resource)
		resultKey2 = fmt.Sprintf("%s-%s-%s-%s", policy, resourceNamespace, kind, resource)
		resultKey3 = fmt.Sprintf("%s-%s-%s-%s", policyNamespace, policy, kind, resource)
		resultKey4 = fmt.Sprintf("%s-%s-%s-%s-%s", policyNamespace, policy, resourceNamespace, kind, resource)
	} else {
		resultKey1 = fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)
		resultKey2 = fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, resourceNamespace, kind, resource)
		resultKey3 = fmt.Sprintf("%s-%s-%s-%s-%s", policyNamespace, policy, rule, kind, resource)
		resultKey4 = fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNamespace, policy, rule, resourceNamespace, kind, resource)
	}

	resultsKey = append(resultsKey, resultKey1, resultKey2, resultKey3, resultKey4)
	return resultsKey
}

func GetResultKeyAccordingToTestResults(policyNs, policy, rule, resourceNs, kind, resource string, isVap bool) string {
	var resultKey string
	if isVap {
		resultKey = fmt.Sprintf("%s-%s-%s", policy, kind, resource)

		if policyNs != "" && resourceNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policyNs, policy, resourceNs, kind, resource)
		} else if policyNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s", policyNs, policy, kind, resource)
		} else if resourceNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s", policy, resourceNs, kind, resource)
		}
	} else {
		resultKey = fmt.Sprintf("%s-%s-%s-%s", policy, rule, kind, resource)

		if policyNs != "" && resourceNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s-%s", policyNs, policy, rule, resourceNs, kind, resource)
		} else if policyNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policyNs, policy, rule, kind, resource)
		} else if resourceNs != "" {
			resultKey = fmt.Sprintf("%s-%s-%s-%s-%s", policy, rule, resourceNs, kind, resource)
		}
	}

	return resultKey
}

func isNamespacedPolicy(policyNames string) (bool, error) {
	return regexp.MatchString("^[a-z]*/[a-z]*", policyNames)
}

// getAndCompareResource --> Get the patchedResource or generatedResource from the path provided by user
// And compare this resource with engine generated resource.
func getAndCompareResource(path string, engineResource unstructured.Unstructured, isGit bool, policyResourcePath string, fs billy.Filesystem, isGenerate bool) string {
	var status string
	resourceType := "patchedResource"
	if isGenerate {
		resourceType = "generatedResource"
	}

	userResource, err := common.GetResourceFromPath(fs, path, isGit, policyResourcePath, resourceType)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		return ""
	}
	matched, err := generate.ValidateResourceWithPattern(log.Log, engineResource.UnstructuredContent(), userResource.UnstructuredContent())
	if err != nil {
		log.Log.V(3).Info(resourceType+" mismatch", "error", err.Error())
		status = "fail"
	} else if matched == "" {
		status = "pass"
	}
	return status
}

func buildMessage(resp engineapi.EngineResponse) string {
	var messages []string
	for _, ruleResp := range resp.PolicyResponse.Rules {
		message := strings.TrimSpace(ruleResp.Message())
		if message != "" {
			messages = append(messages, message)
		}
	}
	return strings.Join(messages, ",")
}

func getUserDefinedPolicyNameAndNamespace(policyName string) (string, string) {
	if strings.Contains(policyName, "/") {
		parts := strings.Split(policyName, "/")
		namespace := parts[0]
		policy := parts[1]
		return namespace, policy
	}
	return "", policyName
}
