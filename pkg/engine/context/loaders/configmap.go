package loaders

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type configMapLoader struct {
	ctx             context.Context //nolint:containedctx
	logger          logr.Logger
	entry           kyvernov1.ContextEntry
	resolver        engineapi.ConfigmapResolver
	enginectx       enginecontext.Interface
	data            []byte
	policyNamespace string
}

func NewConfigMapLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	resolver engineapi.ConfigmapResolver,
	enginectx enginecontext.Interface,
	policyNamespace string,
) enginecontext.Loader {
	return &configMapLoader{
		ctx:             ctx,
		logger:          logger,
		entry:           entry,
		resolver:        resolver,
		enginectx:       enginectx,
		policyNamespace: policyNamespace,
	}
}

func (cml *configMapLoader) HasLoaded() bool {
	return cml.data != nil
}

func (cml *configMapLoader) LoadData() error {
	if cml.resolver == nil {
		return fmt.Errorf("a ConfigmapResolver is required")
	}

	if cml.data == nil {
		data, err := cml.fetchConfigMap()
		if err != nil {
			return fmt.Errorf("failed to retrieve config map for context entry %s: %v", cml.entry.Name, err)
		}

		cml.data = data
	}

	if err := cml.enginectx.AddContextEntry(cml.entry.Name, cml.data); err != nil {
		return fmt.Errorf("failed to add config map for context entry %s: %v", cml.entry.Name, err)
	}

	return nil
}

func (cml *configMapLoader) fetchConfigMap() ([]byte, error) {
	logger := cml.logger
	entryName := cml.entry.Name
	cmName := cml.entry.ConfigMap.Name
	cmNamespace := cml.entry.ConfigMap.Namespace

	contextData := make(map[string]interface{})
	name, err := variables.SubstituteAll(logger, cml.enginectx, cml.entry.ConfigMap.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.name %s: %v", entryName, cmName, err)
	}
	nameStr, ok := name.(string)
	if !ok {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.name %s: expected string, got %T", entryName, cmName, name)
	}
	namespace, err := variables.SubstituteAll(logger, cml.enginectx, cml.entry.ConfigMap.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.namespace %s: %v", entryName, cmNamespace, err)
	}
	namespaceStr, ok := namespace.(string)
	if !ok {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.namespace %s: expected string, got %T", entryName, cmNamespace, namespace)
	}
	if namespaceStr == "" {
		// For namespaced policies, default to the policy's own namespace.
		// For ClusterPolicies (policyNamespace == ""), preserve the existing default of "default".
		if cml.policyNamespace != "" {
			namespaceStr = cml.policyNamespace
		} else {
			namespaceStr = "default"
		}
	}
	// For namespaced policies, reject cross-namespace ConfigMap access.
	// This mirrors the protection applied to apiCall.URLPath in CVE-2026-22039.
	if cml.policyNamespace != "" && namespaceStr != cml.policyNamespace {
		return nil, fmt.Errorf("context entry %s: configMap namespace %q is different from policy namespace %q", entryName, namespaceStr, cml.policyNamespace)
	}
	obj, err := cml.resolver.Get(cml.ctx, namespaceStr, nameStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s : %v", namespaceStr, nameStr, err)
	}
	// extract configmap data
	contextData["data"] = obj.Data
	contextData["metadata"] = obj.ObjectMeta
	data, err := json.Marshal(contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap %s/%s: %v", namespaceStr, nameStr, err)
	}
	return data, nil
}
