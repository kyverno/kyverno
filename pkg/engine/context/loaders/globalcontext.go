package loaders

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
)

type Store interface {
	Get(key string) (store.Entry, bool)
}

type gctxLoader struct {
	ctx       context.Context //nolint:containedctx
	logger    logr.Logger
	entry     kyvernov1.ContextEntry
	enginectx enginecontext.Interface
	jp        jmespath.Interface
	gctxStore Store
	data      []byte
}

func NewGCTXLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	jp jmespath.Interface,
	gctxStore Store,
) enginecontext.Loader {
	return &gctxLoader{
		ctx:       ctx,
		logger:    logger,
		entry:     entry,
		enginectx: enginectx,
		jp:        jp,
		gctxStore: gctxStore,
	}
}

func (g *gctxLoader) HasLoaded() bool {
	data, ok := g.gctxStore.Get(g.entry.Name)
	if ok {
		g.logger.Error(fmt.Errorf("failed to get data from global context store"), "failed to get data from global context store")
		return false
	}
	if data == nil {
		return false
	}
	return true
}

func (g *gctxLoader) LoadData() error {
	contextData, err := g.loadGctxData()
	if err != nil {
		g.logger.Error(err, "failed to marshal APICall data for context entry")
		return fmt.Errorf("failed to marshal APICall data for context entry %s: %w", g.entry.Name, err)
	}

	err = g.enginectx.AddContextEntry(g.entry.Name, contextData)
	if err != nil {
		g.logger.Error(err, "failed to add resource cache results for context entry")
		return fmt.Errorf("failed to add resource cache results for context entry %s: %w", g.entry.Name, err)
	}

	g.logger.V(6).Info("added context data", "name", g.entry.Name, "contextData", contextData)
	g.data = contextData
	return nil
}

func (g *gctxLoader) loadGctxData() ([]byte, error) {
	var data interface{}
	var err error
	if g.entry.GlobalReference == nil {
		g.logger.Error(err, "context entry does not have resource cache")
		return nil, fmt.Errorf("resource cache not found")
	}
	rc, err := variables.SubstituteAllInType(g.logger, g.enginectx, g.entry.GlobalReference)
	if err != nil {
		return nil, err
	}
	g.logger.V(6).Info("variables substituted", "resourcecache", rc)

	storeEntry, ok := g.gctxStore.Get(rc.Name)
	if !ok {
		err := fmt.Errorf("failed to fetch entry key=%s", rc.Name)
		g.logger.Error(err, "")
		return nil, err
	}
	data, err = storeEntry.Get()
	if err != nil {
		g.logger.Error(err, "failed to fetch data from entry")
		return nil, err
	}

	var jsonData []byte
	if _, ok := data.([]byte); ok {
		jsonData = data.([]byte)
	} else {
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}
	g.logger.V(6).Info("fetched json data", "name", g.entry.Name, "jsondata", jsonData)

	if g.entry.GlobalReference.JMESPath == "" {
		err := g.enginectx.AddContextEntry(g.entry.Name, jsonData)
		if err != nil {
			g.logger.Error(err, "failed to add resource data to context entry")
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", g.entry.Name, err)
		}

		return jsonData, nil
	}

	path, err := variables.SubstituteAll(g.logger, g.enginectx, rc.JMESPath)
	if err != nil {
		g.logger.Error(err, "failed to substitute variables in context entry")
		return nil, fmt.Errorf("failed to substitute variables in context entry %s JMESPath %s: %w", g.entry.Name, rc.JMESPath, err)
	}

	results, err := g.applyJMESPathJSON(path.(string), jsonData)
	if err != nil {
		g.logger.Error(err, "failed to apply JMESPath for context entry")
		return nil, fmt.Errorf("failed to apply JMESPath %s for context entry %s: %w", path, g.entry.Name, err)
	}
	g.logger.V(6).Info("applied jmespath expression", "name", g.entry.Name, "results", results)

	return json.Marshal(results)
}

func (a *gctxLoader) applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}
	return a.jp.Search(jmesPath, data)
}
