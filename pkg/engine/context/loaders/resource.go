package loaders

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/resource"
)

type resourceLoader struct {
	ctx       context.Context //nolint:containedctx
	logger    logr.Logger
	entry     kyvernov1.ContextEntry
	enginectx enginecontext.Interface
	resource  resource.Interface
	data      []byte
}

func NewResourceLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	rc resource.Interface,
) enginecontext.Loader {
	return &resourceLoader{
		ctx:       ctx,
		logger:    logger,
		entry:     entry,
		resource:  rc,
		enginectx: enginectx,
	}
}

func (r *resourceLoader) HasLoaded() bool {
	return r.data != nil
}

func (r *resourceLoader) LoadData() error {
	if r.data == nil {
		var err error
		if r.data, err = r.resource.Get(r.entry, r.enginectx); err != nil {
			return fmt.Errorf("failed to fetch data for Resource cache entry: %w", err)
		}
	}
	return nil
}
