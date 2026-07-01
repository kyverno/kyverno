package processor

import (
	"encoding/json"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	clicontext "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/context"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/informers"
)

func policyHasValidateOrVerifyImageChecks(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		//  engine.validate handles both validate and verifyImageChecks atm
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			return true
		}
	}
	return false
}

func NewContextProvider(dclient dclient.Interface, restMapper meta.RESTMapper, f billy.Filesystem, contextPath string, registryAccess bool, isFake bool, globalContextEntries map[string]interface{}, httpMockIndex map[string]interface{}) (libs.Context, error) {
	if dclient != nil && !isFake {
		kubeClient := dclient.GetKubeClient()
		informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)

		// TODO: informer exit, we rely on the fact that this is gonna be used in the cli
		// so the context and the informer will just die at the end of the command execution.
		// but maybe we can do better ?
		stopCh := make(chan struct{})
		informerFactory.Start(stopCh)
		informerFactory.WaitForCacheSync(stopCh)

		lister := informerFactory.Core().V1().Secrets().Lister()
		return libs.NewContextProvider(
			dclient,
			lister,
			gctxstore.New(),
			restMapper,
		)
	}

	fakeContextProvider := libs.NewFakeContextProvider()
	if contextPath != "" {
		ctx, err := clicontext.Load(f, contextPath)
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
		for _, imgData := range ctx.ContextSpec.Images {
			raw, err := json.Marshal(imgData)
			if err != nil {
				return nil, err
			}
			var asMap map[string]any
			if err := json.Unmarshal(raw, &asMap); err != nil {
				return nil, err
			}
			fakeContextProvider.AddImageData(imgData.Image, asMap)
		}
	}

	if len(globalContextEntries) > 0 {
		for name, data := range globalContextEntries {
			fakeContextProvider.AddGlobalReference(name, data)
		}
	}

	if len(httpMockIndex) > 0 {
		fakeContextProvider.SetHTTPMocks(httpMockIndex)
	}

	libs.LibraryContext = fakeContextProvider
	return fakeContextProvider, nil
}
