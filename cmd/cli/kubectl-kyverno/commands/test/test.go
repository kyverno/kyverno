package test

import (
	"fmt"
	"io"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/pluralize"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/path"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func runTest(out io.Writer, testCase test.TestCase, auditWarn bool) ([]engineapi.EngineResponse, error) {
	// don't process test case with errors
	if testCase.Err != nil {
		return nil, testCase.Err
	}
	fmt.Fprintln(out, "Loading test", testCase.Test.Name, "(", testCase.Path, ")", "...")
	isGit := testCase.Fs != nil
	testDir := testCase.Dir()
	var dClient dclient.Interface
	// values/variables
	fmt.Fprintln(out, "  Loading values/variables", "...")
	vars, err := variables.New(out, testCase.Fs, testDir, testCase.Test.Variables, testCase.Test.Values)
	if err != nil {
		err = fmt.Errorf("failed to decode yaml (%w)", err)
		return nil, err
	}
	// user info
	var userInfo *v1beta1.RequestInfo
	if testCase.Test.UserInfo != "" {
		fmt.Fprintln(out, "  Loading user infos", "...")
		info, err := userinfo.Load(testCase.Fs, testCase.Test.UserInfo, testDir)
		if err != nil {
			return nil, fmt.Errorf("Error: failed to load request info (%s)", err)
		}
		deprecations.CheckUserInfo(out, testCase.Test.UserInfo, info)
		userInfo = &info.RequestInfo
	}
	// policies
	fmt.Fprintln(out, "  Loading policies", "...")
	policyFullPath := path.GetFullPaths(testCase.Test.Policies, testDir, isGit)
	policies, validatingAdmissionPolicies, err := policy.Load(testCase.Fs, testDir, policyFullPath...)
	if err != nil {
		return nil, fmt.Errorf("Error: failed to load policies (%s)", err)
	}
	// resources
	fmt.Fprintln(out, "  Loading resources", "...")
	resourceFullPath := path.GetFullPaths(testCase.Test.Resources, testDir, isGit)
	resources, err := common.GetResourceAccordingToResourcePath(out, testCase.Fs, resourceFullPath, false, policies, validatingAdmissionPolicies, dClient, "", false, testDir)
	if err != nil {
		return nil, fmt.Errorf("Error: failed to load resources (%s)", err)
	}
	uniques, duplicates := resource.RemoveDuplicates(resources)
	if len(duplicates) > 0 {
		for dup := range duplicates {
			fmt.Fprintln(out, "  Warning: found duplicated resource", dup.Kind, dup.Name, dup.Namespace)
		}
	}
	// init store
	store.SetLocal(true)
	if vars != nil {
		vars.SetInStore()
	}
	fmt.Fprintln(out, "  Applying", len(policies)+len(validatingAdmissionPolicies), pluralize.Pluralize(len(policies)+len(validatingAdmissionPolicies), "policy", "policies"), "to", len(uniques), pluralize.Pluralize(len(uniques), "resource", "resources"), "...")
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
							fmt.Fprintf(out, "    Error: failed to get unstructured rule (%s)\n", err)
							break
						}
						genClone, _, err := unstructured.NestedMap(ruleUnstr.Object, "clone")
						if err != nil {
							fmt.Fprintf(out, "    Error: failed to read data (%s)\n", err)
							break
						}
						if len(genClone) != 0 {
							if isGit {
								ruleToCloneSourceResource[rule.Name] = res.CloneSourceResource
							} else {
								ruleToCloneSourceResource[rule.Name] = path.GetFullPath(res.CloneSourceResource, testDir)
							}
						}
					}
					break
				}
			}
		}
	}
	// validate policies
	var validPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		// TODO we should return this info to the caller
		_, err := policyvalidation.Validate(pol, nil, nil, true, config.KyvernoUserName(config.KyvernoServiceAccountName()))
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", pol.GetName())
			continue
		}
		validPolicies = append(validPolicies, pol)
	}
	rclient := store.GetRegistryClient()
	if rclient == nil {
		rclient = registryclient.NewOrDie()
	}
	// execute engine
	var engineResponses []engineapi.EngineResponse
	var resultCounts processor.ResultCounts
	for _, resource := range uniques {
		processor := processor.PolicyProcessor{
			Policies:                  validPolicies,
			Resource:                  *resource,
			MutateLogPath:             "",
			Variables:                 vars,
			UserInfo:                  userInfo,
			PolicyReport:              true,
			NamespaceSelectorMap:      vars.NamespaceSelectors(),
			Rc:                        &resultCounts,
			RuleToCloneSourceResource: ruleToCloneSourceResource,
			Client:                    dClient,
			Subresources:              vars.Subresources(),
			Out:                       out,
			RegistryClient:            rclient,
		}
		ers, err := processor.ApplyPoliciesOnResource()
		if err != nil {
			return nil, fmt.Errorf("failed to apply policies on resource %v (%w)", resource.GetName(), err)
		}
		engineResponses = append(engineResponses, ers...)
	}
	for _, resource := range uniques {
		processor := processor.ValidatingAdmissionPolicyProcessor{
			Policies:     validatingAdmissionPolicies,
			Resource:     resource,
			PolicyReport: true,
			Rc:           &resultCounts,
		}
		ers, err := processor.ApplyPolicyOnResource()
		if err != nil {
			return nil, fmt.Errorf("failed to apply policies on resource %s (%w)", resource.GetName(), err)
		}
		engineResponses = append(engineResponses, ers...)
	}
	return engineResponses, nil
}
