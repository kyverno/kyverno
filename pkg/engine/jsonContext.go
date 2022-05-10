package engine

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	jmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// LoadContext - Fetches and adds external data to the Context.
func LoadContext(logger logr.Logger, contextEntries []kyverno.ContextEntry, ctx *PolicyContext, ruleName string) error {
	if len(contextEntries) == 0 {
		return nil
	}

	policyName := ctx.Policy.GetName()
	if store.GetMock() {
		hasRegistryAccess := store.GetRegistryAccess()
		for _, entry := range contextEntries {
			if entry.ImageRegistry != nil && hasRegistryAccess {
				if err := loadImageData(logger, entry, ctx); err != nil {
					return err
				}
			} else if entry.Variable != nil {
				if err := loadVariable(logger, entry, ctx); err != nil {
					return err
				}
			}
		}
		rule := store.GetPolicyRuleFromContext(policyName, ruleName)
		if rule != nil && len(rule.Values) > 0 {
			variables := rule.Values
			for key, value := range variables {
				if err := ctx.JSONContext.AddVariable(key, value); err != nil {
					return err
				}
			}
		}

		if rule != nil && len(rule.ForeachValues) > 0 {
			for key, value := range rule.ForeachValues {
				if err := ctx.JSONContext.AddVariable(key, value[store.ForeachElement]); err != nil {
					return err
				}
			}
		}
	} else {
		for _, entry := range contextEntries {
			if entry.ConfigMap != nil {
				if err := loadConfigMap(logger, entry, ctx); err != nil {
					return err
				}
			} else if entry.APICall != nil {
				if err := loadAPIData(logger, entry, ctx); err != nil {
					return err
				}
			} else if entry.ImageRegistry != nil {
				if err := loadImageData(logger, entry, ctx); err != nil {
					return err
				}
			} else if entry.Variable != nil {
				if err := loadVariable(logger, entry, ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadVariable(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) (err error) {
	path := ""
	if entry.Variable.JMESPath != "" {
		jp, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.Variable.JMESPath)
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
		defaultValue, err = variables.SubstituteAll(logger, ctx.JSONContext, value)
		if err != nil {
			return fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.Variable.Default, err)
		}
		logger.V(4).Info("evaluated default value", "variable name", entry.Name, "jmespath", defaultValue)
	}
	var output interface{} = defaultValue
	if entry.Variable.Value != nil {
		value, _ := variables.DocumentToUntyped(entry.Variable.Value)
		variable, err := variables.SubstituteAll(logger, ctx.JSONContext, value)
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
			if variable, err := ctx.JSONContext.Query(path); err == nil {
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
		return ctx.JSONContext.ReplaceContextEntry(entry.Name, outputBytes)
	} else {
		return fmt.Errorf("unable to add context entry for variable %s: %w", entry.Name, err)
	}
}

func loadImageData(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) error {
	if len(registryclient.Secrets) > 0 {
		if err := registryclient.UpdateKeychain(); err != nil {
			return fmt.Errorf("unable to load image registry credentials, %w", err)
		}
	}
	imageData, err := fetchImageData(logger, entry, ctx)
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(imageData)
	if err != nil {
		return err
	}
	if err := ctx.JSONContext.AddContextEntry(entry.Name, jsonBytes); err != nil {
		return fmt.Errorf("failed to add resource data to context: contextEntry: %v, error: %v", entry, err)
	}
	return nil
}

func fetchImageData(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) (interface{}, error) {
	ref, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.ImageRegistry.Reference)
	if err != nil {
		return nil, fmt.Errorf("ailed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.Reference, err)
	}
	refString, ok := ref.(string)
	if !ok {
		return nil, fmt.Errorf("invalid image reference %s, image reference must be a string", ref)
	}
	path, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.ImageRegistry.JMESPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.JMESPath, err)
	}
	imageData, err := fetchImageDataMap(refString)
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
func fetchImageDataMap(ref string) (interface{}, error) {
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", ref, err)
	}
	desc, err := remote.Get(parsedRef, remote.WithAuthFromKeychain(registryclient.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image reference: %s, error: %v", ref, err)
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
		"resolvedImage": fmt.Sprintf("%s@%s", parsedRef.Context().Name(), desc.Digest.String()),
		"registry":      parsedRef.Context().RegistryStr(),
		"repository":    parsedRef.Context().RepositoryStr(),
		"identifier":    parsedRef.Identifier(),
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

func loadAPIData(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) error {
	jsonData, err := fetchAPIData(logger, entry, ctx)
	if err != nil {
		return err
	}

	if entry.APICall.JMESPath == "" {
		err = ctx.JSONContext.AddContextEntry(entry.Name, jsonData)
		if err != nil {
			return fmt.Errorf("failed to add resource data to context: contextEntry: %v, error: %v", entry, err)
		}

		return nil
	}

	path, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.APICall.JMESPath)
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

	err = ctx.JSONContext.AddContextEntry(entry.Name, contextData)
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

func fetchAPIData(log logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) ([]byte, error) {
	if entry.APICall == nil {
		return nil, fmt.Errorf("missing APICall in context entry %s %v", entry.Name, entry.APICall)
	}

	path, err := variables.SubstituteAll(log, ctx.JSONContext, entry.APICall.URLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.APICall.URLPath, err)
	}

	pathStr := path.(string)
	p, err := NewAPIPath(pathStr)
	if err != nil {
		return nil, fmt.Errorf("failed to build API path for %s %v: %v", entry.Name, entry.APICall, err)
	}

	var jsonData []byte
	if p.Name != "" {
		jsonData, err = loadResource(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource with urlPath: %s: %v", p, err)
		}

	} else {
		jsonData, err = loadResourceList(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource list with urlPath: %s, error: %v", p, err)
		}
	}

	return jsonData, nil
}

func loadResourceList(ctx *PolicyContext, p *APIPath) ([]byte, error) {
	if ctx.Client == nil {
		return nil, fmt.Errorf("API client is not available")
	}

	l, err := ctx.Client.ListResource(p.Version, p.ResourceType, p.Namespace, nil)
	if err != nil {
		return nil, err
	}

	return l.MarshalJSON()
}

func loadResource(ctx *PolicyContext, p *APIPath) ([]byte, error) {
	if ctx.Client == nil {
		return nil, fmt.Errorf("API client is not available")
	}

	r, err := ctx.Client.GetResource(p.Version, p.ResourceType, p.Namespace, p.Name)
	if err != nil {
		return nil, err
	}

	return r.MarshalJSON()
}

func loadConfigMap(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) error {
	data, err := fetchConfigMap(logger, entry, ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve config map for context entry %s: %v", entry.Name, err)
	}

	err = ctx.JSONContext.AddContextEntry(entry.Name, data)
	if err != nil {
		return fmt.Errorf("failed to add config map for context entry %s: %v", entry.Name, err)
	}

	return nil
}

func fetchConfigMap(logger logr.Logger, entry kyverno.ContextEntry, ctx *PolicyContext) ([]byte, error) {
	contextData := make(map[string]interface{})

	name, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.ConfigMap.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.name %s: %v", entry.Name, entry.ConfigMap.Name, err)
	}

	namespace, err := variables.SubstituteAll(logger, ctx.JSONContext, entry.ConfigMap.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.namespace %s: %v", entry.Name, entry.ConfigMap.Namespace, err)
	}

	if namespace == "" {
		namespace = "default"
	}

	obj, err := ctx.Client.GetResource("v1", "ConfigMap", namespace.(string), name.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s : %v", namespace, name, err)
	}

	unstructuredObj := obj.DeepCopy().Object

	// extract configmap data
	contextData["data"] = unstructuredObj["data"]
	contextData["metadata"] = unstructuredObj["metadata"]
	data, err := json.Marshal(contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap %s/%s: %v", namespace, name, err)
	}

	return data, nil
}
