package processor

import (
	"encoding/json"
	"fmt"

	"github.com/go-git/go-billy/v5"
	clicontext "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/context"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/informers"
)

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
		gctxStore := gctxstore.New(0)
		mocks := libs.NewFakeContextProvider()
		for name, data := range globalContextEntries {
			mocks.AddGlobalReference(name, data)
			if err := gctxStore.Set(name, &mockGlobalContextEntry{name: name, mocks: mocks}); err != nil {
				return nil, fmt.Errorf("failed to set global context entry %q: %w", name, err)
			}
		}
		return libs.NewContextProvider(
			dclient,
			lister,
			gctxStore,
			restMapper,
			true,
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

// mockGlobalContextEntry serves a mocked globalContextEntries value from a real
// global context store, delegating to the fake provider for projection semantics.
type mockGlobalContextEntry struct {
	name  string
	mocks *libs.FakeContextProvider
}

func (e *mockGlobalContextEntry) Get(projection string) (any, error) {
	return e.mocks.GetGlobalReference(e.name, projection)
}

func (e *mockGlobalContextEntry) Stop() {}
