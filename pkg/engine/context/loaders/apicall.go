package loaders

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

type apiLoader struct {
	ctx       context.Context //nolint:containedctx
	logger    logr.Logger
	entry     kyvernov1.ContextEntry
	enginectx enginecontext.Interface
	jp        jmespath.Interface
	client    engineapi.RawClient
	config    apicall.APICallConfiguration
	data      []byte
}

func NewAPILoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	jp jmespath.Interface,
	client engineapi.RawClient,
	apiCallConfig apicall.APICallConfiguration,
) enginecontext.Loader {
	return &apiLoader{
		ctx:       ctx,
		logger:    logger,
		entry:     entry,
		enginectx: enginectx,
		jp:        jp,
		client:    client,
		config:    apiCallConfig,
	}
}

func (a *apiLoader) HasLoaded() bool {
	return a.data != nil
}

func (a *apiLoader) LoadData() error {
	executor, err := apicall.New(a.logger, a.jp, a.entry, a.enginectx, a.client, a.config)
	if err != nil {
		return fmt.Errorf("failed to initiaize APICal: %w", err)
	}
	if a.data == nil {
		var err error
		if a.data, err = executor.Fetch(a.ctx); err != nil {
			return fmt.Errorf("failed to fetch data for APICall: %w", err)
		}
	}
	if _, err := executor.Store(a.data); err != nil {
		return fmt.Errorf("failed to store data for APICall: %w", err)
	}
	return nil
}
