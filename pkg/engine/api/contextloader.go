package api

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
)

type RegistryClientFactory interface {
	GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials) (RegistryClient, error)
}

// ContextLoaderFactory provides a ContextLoader given a policy context and rule name
type ContextLoaderFactory = func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(
		ctx context.Context,
		jp jmespath.Interface,
		client RawClient,
		rclientFactory RegistryClientFactory,
		contextEntries []kyvernov1.ContextEntry,
		jsonContext enginecontext.Interface,
	) error
}

func DefaultContextLoaderFactory(
	cmResolver ConfigmapResolver,
) ContextLoaderFactory {
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) ContextLoader {
		return &contextLoader{
			logger:     logging.WithName("DefaultContextLoaderFactory"),
			cmResolver: cmResolver,
		}
	}
}

type contextLoader struct {
	logger     logr.Logger
	cmResolver ConfigmapResolver
}

func (l *contextLoader) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client RawClient,
	rclientFactory RegistryClientFactory,
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
	client RawClient,
	rclientFactory RegistryClientFactory,
	entry kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) enginecontext.DeferredLoader {
	if entry.ConfigMap != nil {
		return func() error {
			if err := LoadConfigMap(ctx, l.logger, entry, jsonContext, l.cmResolver); err != nil {
				return err
			}
			return nil
		}
	} else if entry.APICall != nil {
		return func() error {
			if err := LoadAPIData(ctx, jp, l.logger, entry, jsonContext, client); err != nil {
				return err
			}
			return nil
		}
	} else if entry.ImageRegistry != nil {
		return func() error {
			if err := LoadImageData(ctx, jp, rclientFactory, l.logger, entry, jsonContext); err != nil {
				return err
			}
			return nil
		}
	} else if entry.Variable != nil {
		return func() error {
			if err := LoadVariable(l.logger, jp, entry, jsonContext); err != nil {
				return err
			}
			return nil
		}
	}

	return nil
}
