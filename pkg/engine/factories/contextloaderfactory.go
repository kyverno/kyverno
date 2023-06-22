package factories

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
)

func DefaultContextLoaderFactory(cmResolver engineapi.ConfigmapResolver) engineapi.ContextLoaderFactory {
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.ContextLoader {
		return &contextLoader{
			logger:     logging.WithName("DefaultContextLoaderFactory"),
			cmResolver: cmResolver,
		}
	}
}

type contextLoader struct {
	logger     logr.Logger
	cmResolver engineapi.ConfigmapResolver
}

func (l *contextLoader) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client engineapi.RawClient,
	rclientFactory engineapi.RegistryClientFactory,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	for _, entry := range contextEntries {
		deferredLoader := l.newDeferredLoader(ctx, jp, client, rclientFactory, entry, jsonContext)
		if deferredLoader == nil {
			return fmt.Errorf("invalid context entry %s", entry.Name)
		}
		jsonContext.AddDeferredLoader(entry.Name, deferredLoader)
	}
	return nil
}

func (l *contextLoader) newDeferredLoader(
	ctx context.Context,
	jp jmespath.Interface,
	client engineapi.RawClient,
	rclientFactory engineapi.RegistryClientFactory,
	entry kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) enginecontext.DeferredLoader {
	if entry.ConfigMap != nil {
		return func() error {
			if err := engineapi.LoadConfigMap(ctx, l.logger, entry, jsonContext, l.cmResolver); err != nil {
				return err
			}
			return nil
		}
	} else if entry.APICall != nil {
		return func() error {
			if err := engineapi.LoadAPIData(ctx, jp, l.logger, entry, jsonContext, client); err != nil {
				return err
			}
			return nil
		}
	} else if entry.ImageRegistry != nil {
		return func() error {
			if err := engineapi.LoadImageData(ctx, jp, rclientFactory, l.logger, entry, jsonContext); err != nil {
				return err
			}
			return nil
		}
	} else if entry.Variable != nil {
		return func() error {
			if err := engineapi.LoadVariable(l.logger, jp, entry, jsonContext); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}
