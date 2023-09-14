package processor

import (
	"fmt"
	"io"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func handleGeneratePolicy(out io.Writer, generateResponse *engineapi.EngineResponse, policyContext engine.PolicyContext, ruleToCloneSourceResource map[string]string) ([]engineapi.RuleResponse, error) {
	newResource := policyContext.NewResource()
	objects := []runtime.Object{&newResource}
	for _, rule := range generateResponse.PolicyResponse.Rules {
		if path, ok := ruleToCloneSourceResource[rule.Name()]; ok {
			resourceBytes, err := resource.GetFileBytes(path)
			if err != nil {
				fmt.Fprintf(out, "failed to get resource bytes\n")
			} else {
				r, err := resource.GetUnstructuredResources(resourceBytes)
				if err != nil {
					fmt.Fprintf(out, "failed to convert resource bytes to unstructured format\n")
				}
				for _, res := range r {
					objects = append(objects, res)
				}
			}
		}
	}

	c, err := initializeMockController(out, objects)
	if err != nil {
		fmt.Fprintln(out, "error at controller")
		return nil, err
	}

	gr := kyvernov1beta1.UpdateRequest{
		Spec: kyvernov1beta1.UpdateRequestSpec{
			Type:   kyvernov1beta1.Generate,
			Policy: generateResponse.Policy().GetName(),
			Resource: kyvernov1.ResourceSpec{
				Kind:       generateResponse.Resource.GetKind(),
				Namespace:  generateResponse.Resource.GetNamespace(),
				Name:       generateResponse.Resource.GetName(),
				APIVersion: generateResponse.Resource.GetAPIVersion(),
			},
		},
	}

	var newRuleResponse []engineapi.RuleResponse

	for _, rule := range generateResponse.PolicyResponse.Rules {
		genResource, err := c.ApplyGeneratePolicy(log.Log.V(2), &policyContext, gr, []string{rule.Name()})
		if err != nil {
			return nil, err
		}

		if genResource != nil {
			unstrGenResource, err := c.GetUnstrResource(genResource[0])
			if err != nil {
				return nil, err
			}
			newRuleResponse = append(newRuleResponse, *rule.WithGeneratedResource(*unstrGenResource))
		}
	}

	return newRuleResponse, nil
}

func initializeMockController(out io.Writer, objects []runtime.Object) (*generate.GenerateController, error) {
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, objects...)
	if err != nil {
		fmt.Fprintf(out, "Failed to mock dynamic client")
		return nil, err
	}
	gvrs := sets.New[schema.GroupVersionResource]()
	for _, object := range objects {
		gvk := object.GetObjectKind().GroupVersionKind()
		gvrs.Insert(gvk.GroupVersion().WithResource(strings.ToLower(gvk.Kind) + "s"))
	}
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(gvrs.UnsortedList()))
	cfg := config.NewDefaultConfiguration(false)
	c := generate.NewGenerateControllerWithOnlyClient(client, engine.NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jmespath.New(cfg),
		adapters.Client(client),
		nil,
		imageverifycache.DisabledImageVerifyCache(),
		store.ContextLoaderFactory(nil),
		nil,
		"",
	))
	return c, nil
}
