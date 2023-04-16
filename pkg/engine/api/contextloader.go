package api

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// ContextLoaderFactory provides a ContextLoader given a policy context and rule name
type ContextLoaderFactory = func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(
		ctx context.Context,
		jp jmespath.Interface,
		client dclient.Interface,
		rclient registryclient.Client,
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
	client dclient.Interface,
	rclient registryclient.Client,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	for _, entry := range contextEntries {
		if entry.ConfigMap != nil {
			if err := LoadConfigMap(ctx, l.logger, entry, jsonContext, l.cmResolver); err != nil {
				return err
			}
		} else if entry.APICall != nil {
			if err := LoadAPIData(ctx, jp, l.logger, entry, jsonContext, client); err != nil {
				return err
			}
		} else if entry.ImageRegistry != nil {
			if err := LoadImageData(ctx, jp, rclient, l.logger, entry, jsonContext); err != nil {
				return err
			}
		} else if entry.Variable != nil {
			if err := LoadVariable(l.logger, jp, entry, jsonContext); err != nil {
				return err
			}
		}
	}
	return nil
}
