package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	jmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// LoadContext - Fetches and adds external data to the Context.
func LoadContext(ctx context.Context, logger logr.Logger, rclient registryclient.Client, contextEntries []kyvernov1.ContextEntry, enginectx *PolicyContext, ruleName string) error {
	if len(contextEntries) == 0 {
		return nil
	}

	policyName := enginectx.policy.GetName()
	if store.GetMock() {
		rule := store.GetPolicyRuleFromContext(policyName, ruleName)
		if rule != nil && len(rule.Values) > 0 {
			variables := rule.Values
			for key, value := range variables {
				if err := enginectx.jsonContext.AddVariable(key, value); err != nil {
					return err
				}
			}
		}

		hasRegistryAccess := store.GetRegistryAccess()

		// Context Variable should be loaded after the values loaded from values file
		for _, entry := range contextEntries {
			if entry.ImageRegistry != nil && hasRegistryAccess {
				rclient := store.GetRegistryClient()
				if err := loadImageData(ctx, rclient, logger, entry, enginectx); err != nil {
					return err
				}
			} else if entry.Variable != nil {
				if err := loadVariable(logger, entry, enginectx); err != nil {
					return err
				}
			} else if entry.APICall != nil && store.IsAllowApiCall() {
				if err := loadAPIData(ctx, logger, entry, enginectx); err != nil {
					return err
				}
			}
		}

		if rule != nil && len(rule.ForEachValues) > 0 {
			for key, value := range rule.ForEachValues {
				if err := enginectx.jsonContext.AddVariable(key, value[store.ForeachElement]); err != nil {
					return err
				}
			}
		}
	} else {
		for _, entry := range contextEntries {
			if entry.ConfigMap != nil {
				if err := loadConfigMap(ctx, logger, entry, enginectx); err != nil {
					return err
				}
			} else if entry.APICall != nil {
				if err := loadAPIData(ctx, logger, entry, enginectx); err != nil {
					return err
				}
			} else if entry.ImageRegistry != nil {
				if err := loadImageData(ctx, rclient, logger, entry, enginectx); err != nil {
					return err
				}
			} else if entry.Variable != nil {
				if err := loadVariable(logger, entry, enginectx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadVariable(logger logr.Logger, entry kyvernov1.ContextEntry, ctx *PolicyContext) (err error) {
	path := ""
	if entry.Variable.JMESPath != "" {
		jp, err := variables.SubstituteAll(logger, ctx.jsonContext, entry.Variable.JMESPath)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.JMESPath, err)
		}
		path = jp.(string)
		logger.V(4).Info("evaluated jmespath", "variable name", entry.Name, "jmespath", path)
	}
	var defaultValue interface{} = nil
	if entry.Variable.Default != nil {
		value, err := variables.DocumentToUntyped(entry.Variable.Default)
		if err != nil {
			return fmt.Errorf("invalid default for variable %s", entry.Name)
		}
		defaultValue, err = variables.SubstituteAll(logger, ctx.jsonContext, value)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.Default, err)
		}
		logger.V(4).Info("evaluated default value", "variable name", entry.Name, "jmespath", defaultValue)
	}
	var output interface{} = defaultValue
	if entry.Variable.Value != nil {
		value, _ := variables.DocumentToUntyped(entry.Variable.Value)
		variable, err := variables.SubstituteAll(logger, ctx.jsonContext, value)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.Value, err)
		}
		if path != "" {
			variable, err := applyJMESPath(path, variable)
			if err == nil {
				output = variable
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %s: %v", path, entry.Variable.Value, err)
			}
		} else {
			output = variable
		}
	} else {
		if path != "" {
			if variable, err := ctx.jsonContext.Query(path); err == nil {
				output = variable
			} else if defaultValue == nil {
				return fmt.Errorf("failed to apply jmespath %s to variable %v", path, err)
			}
		}
	}
	logger.V(4).Info("evaluated output", "variable name", entry.Name, "output", output)
	if output == nil {
		return fmt.Errorf("unable to add context entry for variable %s since it evaluated to nil", entry.Name)
	}
	if outputBytes, err := json.Marshal(output); err == nil {
		return ctx.jsonContext.ReplaceContextEntry(entry.Name, outputBytes)
	} else {
		return fmt.Errorf("unable to add context entry for variable %s: %w", entry.Name, err)
	}
}

func loadImageData(ctx context.Context, rclient registryclient.Client, logger logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) error {
	imageData, err := fetchImageData(ctx, rclient, logger, entry, enginectx)
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(imageData)
	if err != nil {
		return err
	}
	if err := enginectx.jsonContext.AddContextEntry(entry.Name, jsonBytes); err != nil {
		return fmt.Errorf("failed to add resource data to context: contextEntry: %v, error: %v", entry, err)
	}
	return nil
}

func fetchImageData(ctx context.Context, rclient registryclient.Client, logger logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) (interface{}, error) {
	ref, err := variables.SubstituteAll(logger, enginectx.jsonContext, entry.ImageRegistry.Reference)
	if err != nil {
		return nil, fmt.Errorf("ailed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.Reference, err)
	}
	refString, ok := ref.(string)
	if !ok {
		return nil, fmt.Errorf("invalid image reference %s, image reference must be a string", ref)
	}
	path, err := variables.SubstituteAll(logger, enginectx.jsonContext, entry.ImageRegistry.JMESPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.JMESPath, err)
	}
	imageData, err := fetchImageDataMap(ctx, rclient, refString)
	if err != nil {
		return nil, err
	}
	if path != "" {
		imageData, err = applyJMESPath(path.(string), imageData)
		if err != nil {
			return nil, fmt.Errorf("failed to apply JMESPath (%s) results to context entry %s, error: %v", entry.ImageRegistry.JMESPath, entry.Name, err)
		}
	}
	return imageData, nil
}

// FetchImageDataMap fetches image information from the remote registry.
func fetchImageDataMap(ctx context.Context, rclient registryclient.Client, ref string) (interface{}, error) {
	desc, err := rclient.FetchImageDescriptor(ctx, ref)
	if err != nil {
		return nil, err
	}
	image, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve image reference: %s, error: %v", ref, err)
	}
	// We need to use the raw config and manifest to avoid dropping unknown keys
	// which are not defined in GGCR structs.
	rawManifest, err := image.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest for image reference: %s, error: %v", ref, err)
	}
	var manifest interface{}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest for image reference: %s, error: %v", ref, err)
	}
	rawConfig, err := image.RawConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config for image reference: %s, error: %v", ref, err)
	}
	var configData interface{}
	if err := json.Unmarshal(rawConfig, &configData); err != nil {
		return nil, fmt.Errorf("failed to decode config for image reference: %s, error: %v", ref, err)
	}

	data := map[string]interface{}{
		"image":         ref,
		"resolvedImage": fmt.Sprintf("%s@%s", desc.Ref.Context().Name(), desc.Digest.String()),
		"registry":      desc.Ref.Context().RegistryStr(),
		"repository":    desc.Ref.Context().RepositoryStr(),
		"identifier":    desc.Ref.Identifier(),
		"manifest":      manifest,
		"configData":    configData,
	}
	// we need to do the conversion from struct types to an interface type so that jmespath
	// evaluation works correctly. go-jmespath cannot handle function calls like max/sum
	// for types like integers for eg. the conversion to untyped allows the stdlib json
	// to convert all the types to types that are compatible with jmespath.
	jsonDoc, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var untyped interface{}
	err = json.Unmarshal(jsonDoc, &untyped)
	if err != nil {
		return nil, err
	}
	return untyped, nil
}

func loadAPIData(ctx context.Context, logger logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) error {
	jsonData, err := fetchAPIData(ctx, logger, entry, enginectx)
	if err != nil {
		return err
	}

	if entry.APICall.JMESPath == "" {
		err = enginectx.jsonContext.AddContextEntry(entry.Name, jsonData)
		if err != nil {
			return fmt.Errorf("failed to add resource data to context: contextEntry: %v, error: %v", entry, err)
		}

		return nil
	}

	path, err := variables.SubstituteAll(logger, enginectx.jsonContext, entry.APICall.JMESPath)
	if err != nil {
		return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.APICall.JMESPath, err)
	}

	results, err := applyJMESPathJSON(path.(string), jsonData)
	if err != nil {
		return err
	}

	contextData, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshall data %v for context entry %v: %v", contextData, entry, err)
	}

	err = enginectx.jsonContext.AddContextEntry(entry.Name, contextData)
	if err != nil {
		return fmt.Errorf("failed to add JMESPath (%s) results to context, error: %v", entry.APICall.JMESPath, err)
	}

	logger.V(4).Info("added APICall context entry", "len", len(contextData))
	return nil
}

func applyJMESPath(jmesPath string, data interface{}) (interface{}, error) {
	jp, err := jmespath.New(jmesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", jmesPath, err)
	}

	return jp.Search(data)
}

func applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %v", string(jsonData), err)
	}
	return applyJMESPath(jmesPath, data)
}

func fetchAPIData(ctx context.Context, log logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) ([]byte, error) {
	if entry.APICall == nil {
		return nil, fmt.Errorf("missing APICall in context entry %s %v", entry.Name, entry.APICall)
	}

	path, err := variables.SubstituteAll(log, enginectx.jsonContext, entry.APICall.URLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.APICall.URLPath, err)
	}

	pathStr := path.(string)

	jsonData, err := getResource(ctx, enginectx, pathStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource with raw url\n: %s: %v", pathStr, err)
	}

	return jsonData, nil
}

func getResource(ctx context.Context, enginectx *PolicyContext, p string) ([]byte, error) {
	return enginectx.client.RawAbsPath(ctx, p)
}

func loadConfigMap(ctx context.Context, logger logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) error {
	data, err := fetchConfigMap(ctx, logger, entry, enginectx)
	if err != nil {
		return fmt.Errorf("failed to retrieve config map for context entry %s: %v", entry.Name, err)
	}

	err = enginectx.jsonContext.AddContextEntry(entry.Name, data)
	if err != nil {
		return fmt.Errorf("failed to add config map for context entry %s: %v", entry.Name, err)
	}

	return nil
}

func fetchConfigMap(ctx context.Context, logger logr.Logger, entry kyvernov1.ContextEntry, enginectx *PolicyContext) ([]byte, error) {
	contextData := make(map[string]interface{})

	name, err := variables.SubstituteAll(logger, enginectx.jsonContext, entry.ConfigMap.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.name %s: %v", entry.Name, entry.ConfigMap.Name, err)
	}

	namespace, err := variables.SubstituteAll(logger, enginectx.jsonContext, entry.ConfigMap.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.namespace %s: %v", entry.Name, entry.ConfigMap.Namespace, err)
	}

	if namespace == "" {
		namespace = "default"
	}

	obj, err := enginectx.informerCacheResolvers.Get(ctx, namespace.(string), name.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s : %v", namespace, name, err)
	}

	// extract configmap data
	contextData["data"] = obj.Data
	contextData["metadata"] = obj.ObjectMeta
	data, err := json.Marshal(contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap %s/%s: %v", namespace, name, err)
	}

	return data, nil
}
