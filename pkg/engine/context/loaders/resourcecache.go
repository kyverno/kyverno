package loaders

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache"
)

type resourceCacheLoader struct {
	ctx           context.Context //nolint:containedctx
	logger        logr.Logger
	entry         kyvernov1.ContextEntry
	enginectx     enginecontext.Interface
	resourceCache resourcecache.ResourceCache
	data          []byte
}

func NewResourceCacheLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	rc resourcecache.ResourceCache,
) enginecontext.Loader {
	return &resourceCacheLoader{
		ctx:           ctx,
		logger:        logger,
		entry:         entry,
		resourceCache: rc,
		enginectx:     enginectx,
	}
}

func (r *resourceCacheLoader) HasLoaded() bool {
	return r.data != nil
}

func (r *resourceCacheLoader) LoadData() error {
	if r.data == nil {
		var err error
		if r.data, err = r.resourceCache.Get(r.entry, r.enginectx); err != nil {
			return fmt.Errorf("failed to fetch data for APICall: %w", err)
		}
	}
	return nil
}
