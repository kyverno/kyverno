package processor

import (
	"fmt"
	"io"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func handleGeneratePolicy(out io.Writer, store *store.Store, generateResponse *engineapi.EngineResponse, policyContext engine.PolicyContext, ruleToCloneSourceResource map[string]string) ([]engineapi.RuleResponse, error) {
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

	listKinds := map[schema.GroupVersionResource]string{}

	// Collect items in a potential cloneList to provide list kinds to the fake dynamic client.
	for _, rule := range autogen.ComputeRules(policyContext.Policy(), "") {
		if !rule.HasGenerate() || len(rule.Generation.CloneList.Kinds) == 0 {
			continue
		}

		for _, kind := range rule.Generation.CloneList.Kinds {
			apiVersion, kind := kubeutils.GetKindFromGVK(kind)

			if apiVersion == "" || kind == "" {
				continue
			}

			gv, err := schema.ParseGroupVersion(apiVersion)
			if err != nil {
				fmt.Fprintf(out, "failed to parse group and version from clone list kind %s: %v\n", apiVersion, err)
				continue
			}

			listKinds[schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: strings.ToLower(kind) + "s",
			}] = kind + "List"
		}
	}

	c, err := initializeMockController(out, store, listKinds, objects)
	if err != nil {
		fmt.Fprintln(out, "error at controller")
		return nil, err
	}

	newRuleResponse := []engineapi.RuleResponse{}

	for _, rule := range generateResponse.PolicyResponse.Rules {
		genResourceMap, err := c.ApplyGeneratePolicy(log.Log.V(2), &policyContext, []string{rule.Name()})
		if err != nil {
			return nil, err
		}
		generatedResources := []kyvernov1.ResourceSpec{}
		for _, v := range genResourceMap {
			generatedResources = append(generatedResources, v...)
		}
		unstrGenResources, err := c.GetUnstrResources(generatedResources)
		if err != nil {
			return nil, err
		}
		newRuleResponse = append(newRuleResponse, *rule.WithGeneratedResources(unstrGenResources))
	}

	return newRuleResponse, nil
}

func initializeMockController(out io.Writer, s *store.Store, gvrToListKind map[schema.GroupVersionResource]string, objects []runtime.Object) (*generate.GenerateController, error) {
	client, err := dclient.NewFakeClient(runtime.NewScheme(), gvrToListKind, objects...)
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
		store.ContextLoaderFactory(s, nil),
		nil,
	))
	return c, nil
}
