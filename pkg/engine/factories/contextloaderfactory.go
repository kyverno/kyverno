package factories

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/loaders"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
)

type ContextLoaderFactoryOptions func(*contextLoader)

func DefaultContextLoaderFactory(opts ...ContextLoaderFactoryOptions) engineapi.ContextLoaderFactory {
	return func(_ kyvernov1.PolicyInterface, _ kyvernov1.Rule) engineapi.ContextLoader {
		cl := &contextLoader{
			logger:       logging.WithName("DefaultContextLoaderFactory"),
			initializers: make([]engineapi.Initializer, 0),
		}
		for _, o := range opts {
			o(cl)
		}
		if cl.jp == nil {
			cl.jp = jmespath.NewWithDefaults()
		}
		return cl
	}
}

// WithAPIClient enables loading of APICall context entries
func WithAPIClient(client engineapi.RawClient) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.client = client
	}
}

// WithRegistryClientFactory enables loading of ImageData context entries
func WithRegistryClientFactory(rcf engineapi.RegistryClientFactory) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.rclientFactory = rcf
	}
}

// WithConfigMapResolver enables loading of ConfigMap context entries
func WithConfigMapResolver(cmResolver engineapi.ConfigmapResolver) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.cmResolver = cmResolver
	}
}

func WithInitializer(initializer engineapi.Initializer) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.initializers = append(cl.initializers, initializer)
	}
}

// WithJMESPath sets the JMESPath engine
func WithJMESPath(jp jmespath.Interface) ContextLoaderFactoryOptions {
	return func(cl *contextLoader) {
		cl.jp = jp
	}
}

type contextLoader struct {
	logger         logr.Logger
	jp             jmespath.Interface
	client         engineapi.RawClient
	rclientFactory engineapi.RegistryClientFactory
	cmResolver     engineapi.ConfigmapResolver
	initializers   []engineapi.Initializer
}

func (l *contextLoader) Load(
	ctx context.Context,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	for _, init := range l.initializers {
		if err := init(jsonContext); err != nil {
			return err
		}
	}

	for _, entry := range contextEntries {
		deferredLoader, err := l.newDeferredLoader(ctx, entry, jsonContext)
		if err != nil {
			return fmt.Errorf("failed to create deferred loader for context entry %s", entry.Name)
		}

		if deferredLoader != nil {
			if err := jsonContext.AddDeferredLoader(deferredLoader); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *contextLoader) newDeferredLoader(
	ctx context.Context,
	entry kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) (enginecontext.DeferredLoader, error) {
	if entry.ConfigMap != nil {
		if l.cmResolver != nil {
			l := loaders.NewConfigMapLoader(ctx, l.logger, entry, l.cmResolver, jsonContext)
			return enginecontext.NewDeferredLoader(entry.Name, l)
		} else {
			l.logger.Info("disabled loading of ConfigMap context entry %s", entry.Name)
			return nil, nil
		}
	} else if entry.APICall != nil {
		if l.client != nil {
			l := loaders.NewAPILoader(ctx, l.logger, entry, jsonContext, l.jp, l.client)
			return enginecontext.NewDeferredLoader(entry.Name, l)
		} else {
			l.logger.Info("disabled loading of APICall context entry %s", entry.Name)
			return nil, nil
		}
	} else if entry.ImageRegistry != nil {
		if l.rclientFactory != nil {
			l := loaders.NewImageDataLoader(ctx, l.logger, entry, jsonContext, l.jp, l.rclientFactory)
			return enginecontext.NewDeferredLoader(entry.Name, l)
		} else {
			l.logger.Info("disabled loading of ImageRegistry context entry %s", entry.Name)
			return nil, nil
		}
	} else if entry.Variable != nil {
		l := loaders.NewVariableLoader(l.logger, entry, jsonContext, l.jp)
		return enginecontext.NewDeferredLoader(entry.Name, l)
	}

	return nil, fmt.Errorf("missing ConfigMap|APICall|ImageRegistry|Variable in context entry %s", entry.Name)
}
