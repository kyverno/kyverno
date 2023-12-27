package processor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	json_patch "github.com/evanphx/json-patch/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"gomodules.xyz/jsonpatch/v2"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyProcessor struct {
	Policies                  []kyvernov1.PolicyInterface
	Resource                  unstructured.Unstructured
	MutateLogPath             string
	MutateLogPathIsDir        bool
	Variables                 *variables.Variables
	UserInfo                  *kyvernov1beta1.RequestInfo
	PolicyReport              bool
	NamespaceSelectorMap      map[string]map[string]string
	Stdin                     bool
	Rc                        *ResultCounts
	PrintPatchResource        bool
	RuleToCloneSourceResource map[string]string
	Client                    dclient.Interface
	AuditWarn                 bool
	Subresources              []v1alpha1.Subresource
	Out                       io.Writer
	RegistryClient            registryclient.Client
}

func (p *PolicyProcessor) ApplyPoliciesOnResource() ([]engineapi.EngineResponse, error) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	resource := p.Resource
	namespaceLabels := p.NamespaceSelectorMap[p.Resource.GetNamespace()]
	var client engineapi.Client
	if p.Client != nil {
		client = adapters.Client(p.Client)
	}

	eng := engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		client,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(p.RegistryClient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		store.ContextLoaderFactory(nil),
		nil,
		"",
	)
	gvk, subresource := resource.GroupVersionKind(), ""
	// If --cluster flag is not set, then we need to find the top level resource GVK and subresource
	if p.Client == nil {
		for _, s := range p.Subresources {
			subgvk := schema.GroupVersionKind{
				Group:   s.Subresource.Group,
				Version: s.Subresource.Version,
				Kind:    s.Subresource.Kind,
			}
			if gvk == subgvk {
				gvk = schema.GroupVersionKind{
					Group:   s.ParentResource.Group,
					Version: s.ParentResource.Version,
					Kind:    s.ParentResource.Kind,
				}
				parts := strings.Split(s.Subresource.Name, "/")
				subresource = parts[1]
			}
		}
	}
	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	var responses []engineapi.EngineResponse
	// mutate
	for _, policy := range p.Policies {
		if !policyHasMutate(policy) {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		mutateResponse := eng.Mutate(context.Background(), policyContext)
		err = p.processMutateEngineResponse(mutateResponse, resPath)
		if err != nil {
			return responses, fmt.Errorf("failed to print mutated result (%w)", err)
		}
		responses = append(responses, mutateResponse)
		resource = mutateResponse.PatchedResource
	}
	// verify images
	for _, policy := range p.Policies {
		if !policyHasVerifyImages(policy) {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		verifyImageResponse, verifiedImageData := eng.VerifyAndPatchImages(context.TODO(), policyContext)
		// update annotation to reflect verified images
		var patches []jsonpatch.JsonPatchOperation
		if !verifiedImageData.IsEmpty() {
			annotationPatches, err := verifiedImageData.Patches(len(verifyImageResponse.PatchedResource.GetAnnotations()) != 0, log.Log)
			if err != nil {
				return responses, err
			}
			// add annotation patches first
			patches = append(annotationPatches, patches...)
		}
		if len(patches) != 0 {
			patch := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
			decoded, err := json_patch.DecodePatch(patch)
			if err != nil {
				return responses, err
			}
			options := &json_patch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
			resourceBytes, err := verifyImageResponse.PatchedResource.MarshalJSON()
			if err != nil {
				return responses, err
			}
			patchedResourceBytes, err := decoded.ApplyWithOptions(resourceBytes, options)
			if err != nil {
				return responses, err
			}
			if err := verifyImageResponse.PatchedResource.UnmarshalJSON(patchedResourceBytes); err != nil {
				return responses, err
			}
		}
		responses = append(responses, verifyImageResponse)
		resource = verifyImageResponse.PatchedResource
	}
	// validate
	for _, policy := range p.Policies {
		if !policyHasValidateOrVerifyImageChecks(policy) {
			continue
		}
		policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
		if err != nil {
			return responses, err
		}
		validateResponse := eng.Validate(context.TODO(), policyContext)
		responses = append(responses, validateResponse)
		resource = validateResponse.PatchedResource
	}
	// generate
	for _, policy := range p.Policies {
		if policyHasGenerate(policy) {
			policyContext, err := p.makePolicyContext(jp, cfg, resource, policy, namespaceLabels, gvk, subresource)
			if err != nil {
				return responses, err
			}
			generateResponse := eng.ApplyBackgroundChecks(context.TODO(), policyContext)
			if !generateResponse.IsEmpty() {
				newRuleResponse, err := handleGeneratePolicy(p.Out, &generateResponse, *policyContext, p.RuleToCloneSourceResource)
				if err != nil {
					log.Log.Error(err, "failed to apply generate policy")
				} else {
					generateResponse.PolicyResponse.Rules = newRuleResponse
				}
				responses = append(responses, generateResponse)
			}
			p.Rc.addGenerateResponse(p.AuditWarn, resPath, generateResponse)
		}
	}
	p.Rc.addEngineResponses(p.AuditWarn, responses...)
	return responses, nil
}

func (p *PolicyProcessor) makePolicyContext(
	jp jmespath.Interface,
	cfg config.Configuration,
	resource unstructured.Unstructured,
	policy kyvernov1.PolicyInterface,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
) (*policycontext.PolicyContext, error) {
	operation := kyvernov1.Create
	var resourceValues map[string]interface{}
	if p.Variables != nil {
		kindOnwhichPolicyIsApplied := common.GetKindsFromPolicy(p.Out, policy, p.Variables.Subresources(), p.Client)
		vals, err := p.Variables.ComputeVariables(policy.GetName(), resource.GetName(), resource.GetKind(), kindOnwhichPolicyIsApplied /*matches...*/)
		if err != nil {
			return nil, fmt.Errorf("policy `%s` have variables. pass the values for the variables for resource `%s` using set/values_file flag (%w)",
				policy.GetName(),
				resource.GetName(),
				err,
			)
		}
		resourceValues = vals
	}
	// TODO: this is kind of buggy, we should read that from the json context
	switch resourceValues["request.operation"] {
	case "DELETE":
		operation = kyvernov1.Delete
	case "UPDATE":
		operation = kyvernov1.Update
	}
	policyContext, err := engine.NewPolicyContext(
		jp,
		resource,
		operation,
		p.UserInfo,
		cfg,
	)
	if err != nil {
		log.Log.Error(err, "failed to create policy context")
		return nil, fmt.Errorf("failed to create policy context (%w)", err)
	}
	if operation == kyvernov1.Update {
		policyContext = policyContext.WithOldResource(resource)
		if err := policyContext.JSONContext().AddOldResource(resource.Object); err != nil {
			return nil, fmt.Errorf("failed to update old resource in json context (%w)", err)
		}
	}
	if operation == kyvernov1.Delete {
		policyContext = policyContext.WithOldResource(resource)
		if err := policyContext.JSONContext().AddOldResource(resource.Object); err != nil {
			return nil, fmt.Errorf("failed to update old resource in json context (%w)", err)
		}
	}
	policyContext = policyContext.
		WithPolicy(policy).
		WithNamespaceLabels(namespaceLabels).
		WithResourceKind(gvk, subresource)
	for key, value := range resourceValues {
		err = policyContext.JSONContext().AddVariable(key, value)
		if err != nil {
			log.Log.Error(err, "failed to add variable to context", "key", key, "value", value)
			return nil, fmt.Errorf("failed to add variable to context %s (%w)", key, err)
		}
	}
	// we need to get the resources back from the context to account for injected variables
	switch operation {
	case kyvernov1.Create:
		ret, err := policyContext.JSONContext().Query("request.object")
		if err != nil {
			return nil, err
		}
		if ret == nil {
			policyContext = policyContext.WithNewResource(unstructured.Unstructured{})
		} else {
			object, ok := ret.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("the object retrieved from the json context is not valid")
			}
			policyContext = policyContext.WithNewResource(unstructured.Unstructured{Object: object})
		}
	case kyvernov1.Update:
		{
			ret, err := policyContext.JSONContext().Query("request.object")
			if err != nil {
				return nil, err
			}
			if ret == nil {
				policyContext = policyContext.WithNewResource(unstructured.Unstructured{})
			} else {
				object, ok := ret.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("the object retrieved from the json context is not valid")
				}
				policyContext = policyContext.WithNewResource(unstructured.Unstructured{Object: object})
			}
		}
		{
			ret, err := policyContext.JSONContext().Query("request.oldObject")
			if err != nil {
				return nil, err
			}
			if ret == nil {
				policyContext = policyContext.WithOldResource(unstructured.Unstructured{})
			} else {
				object, ok := ret.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("the object retrieved from the json context is not valid")
				}
				policyContext = policyContext.WithOldResource(unstructured.Unstructured{Object: object})
			}
		}
	case kyvernov1.Delete:
		ret, err := policyContext.JSONContext().Query("request.oldObject")
		if err != nil {
			return nil, err
		}
		if ret == nil {
			policyContext = policyContext.WithOldResource(unstructured.Unstructured{})
		} else {
			object, ok := ret.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("the object retrieved from the json context is not valid")
			}
			policyContext = policyContext.WithOldResource(unstructured.Unstructured{Object: object})
		}
	}
	return policyContext, nil
}

func (p *PolicyProcessor) processMutateEngineResponse(response engineapi.EngineResponse, resourcePath string) error {
	printMutatedRes := p.Rc.addMutateResponse(resourcePath, response)
	if printMutatedRes && p.PrintPatchResource {
		yamlEncodedResource, err := yamlv2.Marshal(response.PatchedResource.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal (%w)", err)
		}

		if p.MutateLogPath == "" {
			mutatedResource := string(yamlEncodedResource) + string("\n---")
			if len(strings.TrimSpace(mutatedResource)) > 0 {
				if !p.Stdin {
					fmt.Fprintf(p.Out, "\nmutate policy %s applied to %s:", response.Policy().GetName(), resourcePath)
				}
				fmt.Fprintf(p.Out, "\n"+mutatedResource+"\n")
			}
		} else {
			err := p.printMutatedOutput(string(yamlEncodedResource))
			if err != nil {
				return fmt.Errorf("failed to print mutated result (%w)", err)
			}
			fmt.Fprintf(p.Out, "\n\nMutation:\nMutation has been applied successfully. Check the files.")
		}
	}
	return nil
}

func (p *PolicyProcessor) printMutatedOutput(yaml string) error {
	var file *os.File
	mutateLogPath := filepath.Clean(p.MutateLogPath)
	filename := p.Resource.GetName() + "-mutated"
	if !p.MutateLogPathIsDir {
		// truncation for the case when mutateLogPath is a file (not a directory) is handled under pkg/kyverno/apply/test_command.go
		f, err := os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return err
		}
		file = f
	} else {
		f, err := os.OpenFile(filepath.Join(mutateLogPath, filename+".yaml"), os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304
		if err != nil {
			return err
		}
		file = f
	}
	if _, err := file.Write([]byte(yaml + "\n---\n\n")); err != nil {
		if err := file.Close(); err != nil {
			log.Log.Error(err, "failed to close file")
		}
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
