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

type staticEntry struct {
	value any
}

func (e *staticEntry) Get(projection string) (any, error) {
	if projection == "" {
		return e.value, nil
	}
	if m, ok := e.value.(map[string]any); ok {
		if v, found := m[projection]; found {
			return v, nil
		}
		return nil, fmt.Errorf("projection %q not found in global context entry", projection)
	}
	return nil, fmt.Errorf("projection %q requested on non-object value", projection)
}

func (e *staticEntry) Stop() {}

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
		gStore := gctxstore.New(0)
		for name, data := range globalContextEntries {
			_ = gStore.Set(name, &staticEntry{value: data})
		}
		return libs.NewContextProvider(
			dclient,
			lister,
			gStore,
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
