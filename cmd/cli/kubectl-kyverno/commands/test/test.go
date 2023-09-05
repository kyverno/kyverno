package test

import (
	"fmt"

	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/pluralize"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	pathutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/path"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openapi"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func runTest(openApiManager openapi.Manager, testCase test.TestCase, auditWarn bool) ([]engineapi.EngineResponse, error) {
	// don't process test case with errors
	if testCase.Err != nil {
		return nil, testCase.Err
	}
	fmt.Println("Loading test", testCase.Path, "...")
	store.SetLocal(true)
	isGit := testCase.Fs != nil
	testDir := testCase.Dir()
	var dClient dclient.Interface
	// values/variables
	fmt.Println("  Loading values/variables", "...")
	variables, globalValMap, valuesMap, namespaceSelectorMap, subresources, err := common.GetVariable(
		nil,
		testCase.Test.Values,
		testCase.Test.Variables,
		testCase.Fs,
		testDir,
	)
	if err != nil {
		if !sanitizederror.IsErrorSanitized(err) {
			err = sanitizederror.NewWithError("failed to decode yaml", err)
		}
		return nil, err
	}
	// user info
	var userInfo *v1beta1.RequestInfo
	if testCase.Test.UserInfo != "" {
		fmt.Println("  Loading user infos", "...")
		userInfo, err = userinfo.Load(testCase.Fs, testCase.Test.UserInfo, testDir)
		if err != nil {
			return nil, fmt.Errorf("Error: failed to load request info (%s)", err)
		}
	}
	// policies
	fmt.Println("  Loading policies", "...")
	policyFullPath := pathutils.GetFullPaths(testCase.Test.Policies, testDir, isGit)
	policies, validatingAdmissionPolicies, err := policy.Load(testCase.Fs, testDir, policyFullPath...)
	if err != nil {
		return nil, fmt.Errorf("Error: failed to load policies (%s)", err)
	}
	// resources
	fmt.Println("  Loading resources", "...")
	resourceFullPath := pathutils.GetFullPaths(testCase.Test.Resources, testDir, isGit)
	resources, err := common.GetResourceAccordingToResourcePath(testCase.Fs, resourceFullPath, false, policies, validatingAdmissionPolicies, dClient, "", false, isGit, testDir)
	if err != nil {
		return nil, fmt.Errorf("Error: failed to load resources (%s)", err)
	}
	uniques, duplicates := resource.RemoveDuplicates(resources)
	if len(duplicates) > 0 {
		for dup := range duplicates {
			fmt.Println("  Warning: found duplicated resource", dup.Kind, dup.Name, dup.Namespace)
		}
	}
	// TODO document the code below
	ruleToCloneSourceResource := map[string]string{}
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			for _, res := range testCase.Test.Results {
				if res.IsValidatingAdmissionPolicy {
					continue
				}
				if rule.Name == res.Rule {
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
								ruleToCloneSourceResource[rule.Name] = pathutils.GetFullPath(res.CloneSourceResource, testDir)
							}
						}
					}
					break
				}
			}
		}
	}
	// execute engine
	fmt.Println("  Applying", len(policies), pluralize.Pluralize(len(policies), "policy", "policies"), "to", len(uniques), pluralize.Pluralize(len(uniques), "resource", "resources"), "...")
	var engineResponses []engineapi.EngineResponse
	var resultCounts processor.ResultCounts
	// TODO loop through resources first, then through policies second
	for _, policy := range policies {
		// TODO we should return this info to the caller
		_, err := policyvalidation.Validate(policy, nil, nil, true, openApiManager, config.KyvernoUserName(config.KyvernoServiceAccountName()))
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", policy.GetName())
			continue
		}

		matches := common.HasVariables(policy)
		variable := common.RemoveDuplicateAndObjectVariables(matches)

		// TODO
		// 	if len(variable) > 0 {
		// 		if len(variables) == 0 {
		// 			// check policy in variable file
		// 			if valuesFile == "" || valuesMap[policy.GetName()] == nil {
		// 				fmt.Printf("test skipped for policy  %v  (as required variables are not provided by the users) \n \n", policy.GetName())
		// 			}
		// 		}
		// 	}

		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(policy, subresources, dClient)

		for _, resource := range uniques {
			thisPolicyResourceValues, err := common.CheckVariableForPolicy(valuesMap, globalValMap, policy.GetName(), resource.GetName(), resource.GetKind(), variables, kindOnwhichPolicyIsApplied, variable)
			if err != nil {
				message := fmt.Sprintf(
					"policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag",
					policy.GetName(),
					resource.GetName(),
				)
				return nil, sanitizederror.NewWithError(message, err)
			}
			processor := processor.Processor{
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
			ers, err := processor.ApplyPolicyOnResource()
			if err != nil {
				message := fmt.Sprintf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName())
				return nil, sanitizederror.NewWithError(message, err)
			}
			engineResponses = append(engineResponses, ers...)
		}
	}
	validatingAdmissionPolicy := processor.ValidatingAdmissionPolicies{}
	for _, policy := range validatingAdmissionPolicies {
		for _, resource := range uniques {
			applyPolicyConfig := processor.Processor{
				ValidatingAdmissionPolicy: policy,
				Resource:                  resource,
				PolicyReport:              true,
				Rc:                        &resultCounts,
				Client:                    dClient,
				Subresources:              subresources,
			}
			ers, err := validatingAdmissionPolicy.ApplyPolicyOnResource(applyPolicyConfig)
			if err != nil {
				message := fmt.Sprintf("failed to apply policy %v on resource %v", policy.GetName(), resource.GetName())
				return nil, sanitizederror.NewWithError(message, err)
			}
			engineResponses = append(engineResponses, ers...)
		}
	}
	return engineResponses, nil
}
