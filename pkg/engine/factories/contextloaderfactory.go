package factories

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/loaders"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
)

type ContextLoaderFactoryOptions func(*contextLoader)

func DefaultContextLoaderFactory(cmResolver engineapi.ConfigmapResolver, opts ...ContextLoaderFactoryOptions) engineapi.ContextLoaderFactory {
	return func(_ kyvernov1.PolicyInterface, _ kyvernov1.Rule) engineapi.ContextLoader {
		cl := &contextLoader{
			logger:     logging.WithName("DefaultContextLoaderFactory"),
			cmResolver: cmResolver,
		}
		for _, o := range opts {
			o(cl)
		}
		return cl
	}
}

func WithInitializer(initializer engineapi.Initializer) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.initializers = append(cl.initializers, initializer)
	}
}

func WithAPICallConfig(config apicall.APICallConfiguration) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.apiCallConfig = config
	}
}

func WithGlobalContextStore(gctxStore loaders.Store) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.gctxStore = gctxStore
	}
}

type contextLoader struct {
	logger        logr.Logger
	cmResolver    engineapi.ConfigmapResolver
	initializers  []engineapi.Initializer
	apiCallConfig apicall.APICallConfiguration
	gctxStore     loaders.Store
}

func (l *contextLoader) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client engineapi.RawClient,
	rclientFactory engineapi.RegistryClientFactory,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	for _, init := range l.initializers {
		if err := init(jsonContext); err != nil {
			return err
		}
	}
	for _, entry := range contextEntries {
		loader, err := l.newLoader(ctx, jp, client, rclientFactory, entry, jsonContext, l.gctxStore)
		if err != nil {
			return fmt.Errorf("failed to create deferred loader for context entry %s", entry.Name)
		}
		if loader != nil {
			if toggle.FromContext(ctx).EnableDeferredLoading() {
				if err := jsonContext.AddDeferredLoader(loader); err != nil {
					return err
				}
			} else {
				if err := loader.LoadData(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (l *contextLoader) newLoader(
	ctx context.Context,
	jp jmespath.Interface,
	client engineapi.RawClient,
	rclientFactory engineapi.RegistryClientFactory,
	entry kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
	gctx loaders.Store,
) (enginecontext.DeferredLoader, error) {
	if entry.ConfigMap != nil {
		if l.cmResolver != nil {
			ldr := loaders.NewConfigMapLoader(ctx, l.logger, entry, l.cmResolver, jsonContext)
			return enginecontext.NewDeferredLoader(entry.Name, ldr, l.logger)
		} else {
			l.logger.Info("disabled loading of ConfigMap context entry", "name", entry.Name)
			return nil, nil
		}
	} else if entry.APICall != nil {
		if client != nil {
			ldr := loaders.NewAPILoader(ctx, l.logger, entry, jsonContext, jp, client, l.apiCallConfig)
			return enginecontext.NewDeferredLoader(entry.Name, ldr, l.logger)
		} else {
			l.logger.Info("disabled loading of APICall context entry", "name", entry.Name)
			return nil, nil
		}
	} else if entry.GlobalReference != nil {
		if gctx != nil {
			ldr := loaders.NewGCTXLoader(ctx, l.logger, entry, jsonContext, jp, gctx)
			return enginecontext.NewDeferredLoader(entry.Name, ldr, l.logger)
		} else {
			l.logger.Info("disabled loading of GlobalContext context entry", "name", entry.Name)
			return nil, nil
		}
	} else if entry.ImageRegistry != nil {
		if rclientFactory != nil {
			ldr := loaders.NewImageDataLoader(ctx, l.logger, entry, jsonContext, jp, rclientFactory)
			return enginecontext.NewDeferredLoader(entry.Name, ldr, l.logger)
		} else {
			l.logger.Info("disabled loading of ImageRegistry context entry", "name", entry.Name)
			return nil, nil
		}
	} else if entry.Variable != nil {
		ldr := loaders.NewVariableLoader(l.logger, entry, jsonContext, jp)
		return enginecontext.NewDeferredLoader(entry.Name, ldr, l.logger)
	}
	return nil, fmt.Errorf("missing ConfigMap|APICall|ImageRegistry|Variable in context entry %s", entry.Name)
}
