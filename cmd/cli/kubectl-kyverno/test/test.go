package test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	filterutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/filter"
	pathutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/path"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	unstructuredutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/unstructured"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func applyPoliciesFromPath(
	fs billy.Filesystem,
	apiTest *api.Test,
	isGit bool,
	policyResourcePath string,
	rc *resultCounts,
	openApiManager openapi.Manager,
	filter filterutils.Filter,
	auditWarn bool,
) ([]api.TestResults, []engineapi.EngineResponse, error) {
	engineResponses := make([]engineapi.EngineResponse, 0)
	var dClient dclient.Interface
	var resultCounts common.ResultCounts

	store.SetLocal(true)

	var filteredResults []api.TestResults
	for _, res := range apiTest.Results {
		if filter.Apply(res) {
			filteredResults = append(filteredResults, res)
		}
	}
	apiTest.Results = filteredResults

	if len(apiTest.Results) == 0 {
		return nil, nil, nil
	}

	fmt.Printf("\nExecuting %s...\n", apiTest.Name)
	valuesFile := apiTest.Variables
	userInfoFile := apiTest.UserInfo

	variables, globalValMap, valuesMap, namespaceSelectorMap, subresources, err := common.GetVariable(nil, apiTest.Values, apiTest.Variables, fs, isGit, policyResourcePath)
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

	policyFullPath := pathutils.GetFullPaths(apiTest.Policies, policyResourcePath, isGit)
	resourceFullPath := pathutils.GetFullPaths(apiTest.Resources, policyResourcePath, isGit)

	policies, validatingAdmissionPolicies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}

	var filteredPolicies []kyvernov1.PolicyInterface
	for _, p := range policies {
		for _, res := range apiTest.Results {
			if p.GetName() == res.Policy {
				filteredPolicies = append(filteredPolicies, p)
				break
			}
		}
	}

	var filteredVAPs []v1alpha1.ValidatingAdmissionPolicy
	for _, p := range validatingAdmissionPolicies {
		for _, res := range apiTest.Results {
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
			for _, res := range apiTest.Results {
				if res.IsValidatingAdmissionPolicy {
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
							if isGit {
								ruleToCloneSourceResource[rule.Name] = res.CloneSourceResource
							} else {
								ruleToCloneSourceResource[rule.Name] = pathutils.GetFullPath(res.CloneSourceResource, policyResourcePath)
							}
						}
					}
					break
				}
			}
		}
		p.GetSpec().SetRules(filteredRules)
	}
	policies = filteredPolicies

	resources, err := common.GetResourceAccordingToResourcePath(fs, resourceFullPath, false, policies, validatingAdmissionPolicies, dClient, "", false, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load resources\nCause: %s\n", err)
		os.Exit(1)
	}

	checkableResources := selectResourcesForCheck(resources, apiTest)

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
	return apiTest.Results, engineResponses, nil
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

// getAndCompareResource --> Get the patchedResource or generatedResource from the path provided by user
// And compare this resource with engine generated resource.
func getAndCompareResource(
	path string,
	actualResource unstructured.Unstructured,
	fs billy.Filesystem,
	policyResourcePath string,
	isGenerate bool,
) (bool, error) {
	resourceType := "patchedResource"
	if isGenerate {
		resourceType = "generatedResource"
	}
	// TODO fix the way we handle git vs non-git paths (probably at the loading phase)
	if fs == nil {
		path = filepath.Join(policyResourcePath, path)
	}
	expectedResource, err := common.GetResourceFromPath(fs, path, fs != nil, policyResourcePath, resourceType)
	if err != nil {
		return false, fmt.Errorf("Error: failed to load resources (%s)", err)
	}
	if isGenerate {
		unstructuredutils.FixupGenerateLabels(actualResource)
		unstructuredutils.FixupGenerateLabels(expectedResource)
	}
	equals, err := unstructuredutils.Compare(actualResource, expectedResource, true)
	if err != nil {
		return false, fmt.Errorf("Error: failed to compare resources (%s)", err)
	}
	return equals, nil
}
