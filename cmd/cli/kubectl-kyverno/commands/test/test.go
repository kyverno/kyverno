package test

import (
	"context"
	"fmt"
	"io"

	payload "github.com/kyverno/kyverno-json/pkg/payload"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	clicontext "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/context"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/path"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/userinfo"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/ext/output/pluralize"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

type TestResponse struct {
	Trigger map[string][]engineapi.EngineResponse
	Target  map[string][]engineapi.EngineResponse
}

func runTest(out io.Writer, testCase test.TestCase, registryAccess bool) (*TestResponse, error) {
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
	var userInfo *kyvernov2.RequestInfo
	if testCase.Test.UserInfo != "" {
		fmt.Fprintln(out, "  Loading user infos", "...")
		info, err := userinfo.Load(testCase.Fs, testCase.Test.UserInfo, testDir)
		if err != nil {
			return nil, fmt.Errorf("error: failed to load request info (%s)", err)
		}
		deprecations.CheckUserInfo(out, testCase.Test.UserInfo, info)
		userInfo = &info.RequestInfo
	}
	// policies
	fmt.Fprintln(out, "  Loading policies", "...")
	policyFullPath := path.GetFullPaths(testCase.Test.Policies, testDir, isGit)
	results, err := policy.Load(testCase.Fs, testDir, policyFullPath...)
	if err != nil {
		return nil, fmt.Errorf("error: failed to load policies (%s)", err)
	}
	// resources
	fmt.Fprintln(out, "  Loading resources", "...")
	resourceFullPath := path.GetFullPaths(testCase.Test.Resources, testDir, isGit)
	resources, err := common.GetResourceAccordingToResourcePath(out, testCase.Fs, resourceFullPath, false, results.Policies, results.VAPs, dClient, "", false, testDir)
	if err != nil {
		return nil, fmt.Errorf("error: failed to load resources (%s)", err)
	}
	uniques, duplicates := resource.RemoveDuplicates(resources)
	if len(duplicates) > 0 {
		for dup := range duplicates {
			fmt.Fprintln(out, "  warning: found duplicated resource", dup.Kind, dup.Name, dup.Namespace)
		}
	}

	var json any
	if testCase.Test.JSONPayload != "" {
		// JSON payload
		fmt.Fprintln(out, "  Loading JSON payload", "...")
		jsonFullPath := path.GetFullPaths([]string{testCase.Test.JSONPayload}, testDir, isGit)
		json, err = payload.Load(jsonFullPath[0])
		if err != nil {
			return nil, fmt.Errorf("error: failed to load JSON payload (%s)", err)
		}
	}
	targetResourcesPath := path.GetFullPaths(testCase.Test.TargetResources, testDir, isGit)
	targetResources, err := common.GetResourceAccordingToResourcePath(out, testCase.Fs, targetResourcesPath, false, results.Policies, results.VAPs, dClient, "", false, testDir)
	if err != nil {
		return nil, fmt.Errorf("error: failed to load target resources (%s)", err)
	}

	targets := []runtime.Object{}
	for _, t := range targetResources {
		targets = append(targets, t)
	}

	// this will be a dclient containing all target resources. a policy may not do anything with any targets in these
	dClient, err = dclient.NewFakeClient(runtime.NewScheme(), map[schema.GroupVersionResource]string{}, targets...)
	if err != nil {
		return nil, err
	}
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	// exceptions
	fmt.Fprintln(out, "  Loading exceptions", "...")
	exceptionFullPath := path.GetFullPaths(testCase.Test.PolicyExceptions, testDir, isGit)
	polexLoader, err := exception.Load(exceptionFullPath...)
	if err != nil {
		return nil, fmt.Errorf("error: failed to load exceptions (%s)", err)
	}
	// Validates that exceptions cannot be used with ValidatingAdmissionPolicies.
	if len(results.VAPs) > 0 && len(polexLoader.Exceptions) > 0 {
		return nil, fmt.Errorf("error: use of exceptions with ValidatingAdmissionPolicies is not supported")
	}
	// init store
	var store store.Store
	store.SetLocal(true)
	store.SetRegistryAccess(registryAccess)
	if vars != nil {
		vars.SetInStore(&store)
	}

	policyCount := len(results.Policies) + len(results.VAPs)
	policyPlural := pluralize.Pluralize(len(results.Policies)+len(results.VAPs), "policy", "policies")
	resourceCount := len(uniques)
	resourcePlural := pluralize.Pluralize(len(uniques), "resource", "resources")
	if polexLoader != nil {
		exceptionCount := len(polexLoader.Exceptions)
		exceptionCount += len(polexLoader.CELExceptions)
		exceptionsPlural := pluralize.Pluralize(exceptionCount, "exception", "exceptions")
		fmt.Fprintln(out, "  Applying", policyCount, policyPlural, "to", resourceCount, resourcePlural, "with", exceptionCount, exceptionsPlural, "...")
	} else {
		fmt.Fprintln(out, "  Applying", policyCount, policyPlural, "to", resourceCount, resourcePlural, "...")
	}

	// TODO document the code below
	ruleToCloneSourceResource := map[string]string{}
	for _, policy := range results.Policies {
		for _, rule := range autogen.Default.ComputeRules(policy, "") {
			for _, res := range testCase.Test.Results {
				if res.IsValidatingAdmissionPolicy || res.IsValidatingPolicy {
					continue
				}
				// TODO: what if two policies have a rule with the same name ?
				if rule.Name == res.Rule {
					if rule.HasGenerate() {
						if len(rule.Generation.CloneList.Kinds) != 0 { // cloneList
							// We cannot cast this to an unstructured object because it doesn't have a kind.
							if isGit {
								ruleToCloneSourceResource[rule.Name] = res.CloneSourceResource
							} else {
								ruleToCloneSourceResource[rule.Name] = path.GetFullPath(res.CloneSourceResource, testDir)
							}
						} else { // clone or data
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
					}
					break
				}
			}
		}
	}
	// validate policies
	validPolicies := make([]kyvernov1.PolicyInterface, 0, len(results.Policies))
	for _, pol := range results.Policies {
		// TODO we should return this info to the caller
		sa := config.KyvernoUserName(config.KyvernoServiceAccountName())
		_, err := policyvalidation.Validate(pol, nil, nil, true, sa, sa)
		if err != nil {
			log.Log.Error(err, "skipping invalid policy", "name", pol.GetName())
			continue
		}
		validPolicies = append(validPolicies, pol)
	}
	// execute engine
	var engineResponses []engineapi.EngineResponse
	var resultCounts processor.ResultCounts
	testResponse := TestResponse{
		Trigger: map[string][]engineapi.EngineResponse{},
		Target:  map[string][]engineapi.EngineResponse{},
	}
	for _, resource := range uniques {
		// the policy processor is for multiple policies at once
		processor := processor.PolicyProcessor{
			Store:                     &store,
			Policies:                  validPolicies,
			Resource:                  *resource,
			PolicyExceptions:          polexLoader.Exceptions,
			MutateLogPath:             "",
			Variables:                 vars,
			UserInfo:                  userInfo,
			PolicyReport:              true,
			NamespaceSelectorMap:      vars.NamespaceSelectors(),
			Rc:                        &resultCounts,
			RuleToCloneSourceResource: ruleToCloneSourceResource,
			Cluster:                   false,
			Client:                    dClient,
			Subresources:              vars.Subresources(),
			Out:                       io.Discard,
		}
		ers, err := processor.ApplyPoliciesOnResource()
		if err != nil {
			return nil, fmt.Errorf("failed to apply policies on resource %v (%w)", resource.GetName(), err)
		}
		resourceKey := generateResourceKey(resource)
		engineResponses = append(engineResponses, ers...)
		testResponse.Trigger[resourceKey] = ers
	}
	for _, targetResource := range targetResources {
		for _, engineResponse := range engineResponses {
			if r, _ := extractPatchedTargetFromEngineResponse(targetResource.GetAPIVersion(), targetResource.GetKind(), targetResource.GetName(), targetResource.GetNamespace(), engineResponse); r != nil {
				resourceKey := generateResourceKey(targetResource)
				testResponse.Target[resourceKey] = append(testResponse.Target[resourceKey], engineResponse)
			}
		}
	}

	for _, resource := range uniques {
		processor := processor.ValidatingAdmissionPolicyProcessor{
			Policies:             results.VAPs,
			Bindings:             results.VAPBindings,
			Resource:             resource,
			NamespaceSelectorMap: vars.NamespaceSelectors(),
			PolicyReport:         true,
			Rc:                   &resultCounts,
		}
		ers, err := processor.ApplyPolicyOnResource()
		if err != nil {
			return nil, fmt.Errorf("failed to apply policies on resource %s (%w)", resource.GetName(), err)
		}
		resourceKey := generateResourceKey(resource)
		testResponse.Trigger[resourceKey] = append(testResponse.Trigger[resourceKey], ers...)
	}
	if len(results.ValidatingPolicies) != 0 {
		ctx := context.TODO()
		compiler := celpolicy.NewCompiler()
		provider, err := engine.NewProvider(compiler, results.ValidatingPolicies, polexLoader.CELExceptions)
		if err != nil {
			return nil, err
		}
		eng := engine.NewEngine(provider, vars.Namespace, matching.NewMatcher())
		var restMapper meta.RESTMapper
		var contextProvider celpolicy.Context
		apiGroupResources, err := data.APIGroupResources()
		if err != nil {
			return nil, err
		}
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
		fakeContextProvider := celpolicy.NewFakeContextProvider()
		if testCase.Test.Context != "" {
			ctx, err := clicontext.Load(nil, testCase.Test.Context)
			if err != nil {
				return nil, err
			}
			for _, resource := range ctx.ContextSpec.Resources {
				gvk := resource.GroupVersionKind()
				mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
				if err != nil {
					return nil, err
				}
				if err := fakeContextProvider.AddResource(mapping.Resource, &resource); err != nil {
					return nil, err
				}
			}
		}
		contextProvider = fakeContextProvider
		for _, resource := range uniques {
			// get gvk from resource
			gvk := resource.GroupVersionKind()
			// map gvk to gvr
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
			}
			gvr := mapping.Resource
			var user authenticationv1.UserInfo
			if userInfo != nil {
				user = userInfo.AdmissionUserInfo
			}
			// create engine request
			request := engine.Request(
				contextProvider,
				gvk,
				gvr,
				// TODO: how to manage subresource ?
				"",
				resource.GetName(),
				resource.GetNamespace(),
				// TODO: how to manage other operations ?
				admissionv1.Create,
				user,
				resource,
				nil,
				false,
				nil,
			)
			reps, err := eng.Handle(ctx, request)
			if err != nil {
				return nil, fmt.Errorf("failed to apply validating policies on resource %s (%w)", resource.GetName(), err)
			}
			resourceKey := generateResourceKey(resource)
			for _, r := range reps.Policies {
				engineResponse := engineapi.EngineResponse{
					Resource: *reps.Resource,
					PolicyResponse: engineapi.PolicyResponse{
						Rules: r.Rules,
					},
				}
				engineResponse = engineResponse.WithPolicy(engineapi.NewValidatingPolicy(&r.Policy))
				testResponse.Trigger[resourceKey] = append(testResponse.Trigger[resourceKey], engineResponse)
			}
		}

		if json != nil {
			eng = engine.NewEngine(provider, nil, nil)
			request := engine.RequestFromJSON(contextProvider, &unstructured.Unstructured{Object: json.(map[string]interface{})})
			reps, err := eng.Handle(ctx, request)
			if err != nil {
				return nil, fmt.Errorf("failed to apply validating policies on JSON payload %s (%w)", testCase.Test.JSONPayload, err)
			}
			for _, r := range reps.Policies {
				engineResponse := engineapi.EngineResponse{
					Resource: *reps.Resource,
					PolicyResponse: engineapi.PolicyResponse{
						Rules: r.Rules,
					},
				}
				engineResponse = engineResponse.WithPolicy(engineapi.NewValidatingPolicy(&r.Policy))
				testResponse.Trigger[testCase.Test.JSONPayload] = append(testResponse.Trigger[testCase.Test.JSONPayload], engineResponse)
			}
		}
	}
	// this is an array of responses of all policies, generated by all of their rules
	return &testResponse, nil
}

func generateResourceKey(resource *unstructured.Unstructured) string {
	return resource.GetAPIVersion() + "," + resource.GetKind() + "," + resource.GetNamespace() + "," + resource.GetName()
}
