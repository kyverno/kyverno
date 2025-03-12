package processor

// import (
// 	"context"
// 	"fmt"

// 	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
// 	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
// 	"github.com/kyverno/kyverno/pkg/cel/engine"
// 	"github.com/kyverno/kyverno/pkg/cel/matching"
// 	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
// 	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
// 	"github.com/kyverno/kyverno/pkg/clients/dclient"
// 	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
// 	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
// 	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
// 	corev1 "k8s.io/api/core/v1"
// 	"k8s.io/apimachinery/pkg/api/meta"
// 	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
// 	"k8s.io/client-go/restmapper"
// )

// type ValidatingPolicyProcessor struct {
// 	engine          engine.Engine
// 	contextProvider celpolicy.Context
// 	Resource        *unstructured.Unstructured
// 	JsonPayload     *unstructured.Unstructured
// 	Rc              *ResultCounts
// }

// func NewValidatingPolicyProcessor(
// 	Policies []policiesv1alpha1.ValidatingPolicy,
// 	Exceptions []*policiesv1alpha1.CELPolicyException,
// 	NamespaceProvider func(string) *corev1.Namespace,
// 	Client dclient.Interface,
// ) (*ValidatingPolicyProcessor, error) {
// 	compiler := celpolicy.NewCompiler()
// 	provider, err := engine.NewProvider(compiler, p.Policies, p.Exceptions)
// 	if err != nil {
// 		return nil, err
// 	}
// 	eng := engine.NewEngine(provider, p.NamespaceProvider, matching.NewMatcher())
// 	// TODO: mock when no cluster provided
// 	gctxStore := gctxstore.New()
// 	var restMapper meta.RESTMapper
// 	var contextProvider celpolicy.Context
// 	if p.Client != nil {
// 		contextProvider, err = celpolicy.NewContextProvider(
// 			p.Client,
// 			[]imagedataloader.Option{imagedataloader.WithLocalCredentials(c.RegistryAccess)},
// 			gctxStore,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}
// 		apiGroupResources, err := restmapper.GetAPIGroupResources(p.Client.GetKubeClient().Discovery())
// 		if err != nil {
// 			return nil, err
// 		}
// 		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
// 	} else {
// 		apiGroupResources, err := data.APIGroupResources()
// 		if err != nil {
// 			return nil, err
// 		}
// 		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
// 		fakeContextProvider := celpolicy.NewFakeContextProvider()
// 		if c.ContextPath != "" {
// 			ctx, err := clicontext.Load(nil, c.ContextPath)
// 			if err != nil {
// 				return nil, err
// 			}
// 			for _, resource := range ctx.ContextSpec.Resources {
// 				gvk := resource.GroupVersionKind()
// 				mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
// 				if err != nil {
// 					return nil, err
// 				}
// 				if err := fakeContextProvider.AddResource(mapping.Resource, &resource); err != nil {
// 					return nil, err
// 				}
// 			}
// 		}
// 		contextProvider = fakeContextProvider
// 	}
// 	return &ValidatingPolicyProcessor{}
// }

// func (p *ValidatingPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
// 	ctx := context.TODO()
// 	responses := make([]engineapi.EngineResponse, 0)
// 	responsesTemp := make([]engine.EngineResponse, 0)
// 	for _, resource := range resources {
// 		// get gvk from resource
// 		gvk := resource.GroupVersionKind()
// 		// map gvk to gvr
// 		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
// 		if err != nil {
// 			if c.ContinueOnFail {
// 				fmt.Printf("failed to map gvk to gvr %s (%v)\n", gvk, err)
// 				continue
// 			}
// 			return responses, fmt.Errorf("failed to map gvk to gvr %s (%v)\n", gvk, err)
// 		}
// 		gvr := mapping.Resource
// 		// create engine request
// 		request := engine.Request(
// 			contextProvider,
// 			gvk,
// 			gvr,
// 			// TODO: how to manage subresource ?
// 			"",
// 			resource.GetName(),
// 			resource.GetNamespace(),
// 			// TODO: how to manage other operations ?
// 			admissionv1.Create,
// 			resource,
// 			nil,
// 			false,
// 			nil,
// 		)
// 		reps, err := eng.Handle(ctx, request)
// 		if err != nil {
// 			if c.ContinueOnFail {
// 				fmt.Printf("failed to apply validating policies on resource %s (%v)\n", resource.GetName(), err)
// 				continue
// 			}
// 			return responses, fmt.Errorf("failed to apply validating policies on resource %s (%w)", resource.GetName(), err)
// 		}
// 		responsesTemp = append(responsesTemp, reps)
// 	}

// 	for _, json := range jsonPayloads {
// 		eng = engine.NewEngine(provider, nil, nil)
// 		request := engine.RequestFromJSON(contextProvider, json)
// 		reps, err := eng.Handle(ctx, request)
// 		if err != nil {
// 			if c.ContinueOnFail {
// 				fmt.Printf("failed to apply validating policies on JSON payloads (%v)\n", err)
// 				continue
// 			}
// 			return responses, fmt.Errorf("failed to apply validating policies on JSON payloads (%w)", err)
// 		}
// 		responsesTemp = append(responsesTemp, reps)
// 	}
// 	// transform response into legacy engine responses
// 	for _, response := range responsesTemp {
// 		for _, r := range response.Policies {
// 			engineResponse := engineapi.EngineResponse{
// 				Resource: *response.Resource,
// 				PolicyResponse: engineapi.PolicyResponse{
// 					Rules: r.Rules,
// 				},
// 			}
// 			engineResponse = engineResponse.WithPolicy(engineapi.NewValidatingPolicy(&r.Policy))
// 			p.Rc.AddValidatingPolicyResponse(engineResponse)
// 			responses = append(responses, engineResponse)
// 		}
// 	}
// 	return responses, nil
// }
