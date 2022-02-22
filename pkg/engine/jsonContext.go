package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	pkgcommon "github.com/kyverno/kyverno/pkg/common"
	jmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// LoadContext - Fetches and adds external data to the Context.
func LoadContext(logger logr.Logger, contextEntries []kyverno.ContextEntry, ctx *PolicyContext, ruleName string) error {
	if len(contextEntries) == 0 {
		return nil
	}

	policyName := ctx.Policy.Name
	if store.GetMock() {
		rule := store.GetPolicyRuleFromContext(policyName, ruleName)
		if rule == nil || len(rule.Values) == 0 {
			return fmt.Errorf("No values found for policy %s rule %s", policyName, ruleName)
		}
		variables := rule.Values

		for key, value := range variables {
			if trimmedTypedValue := strings.Trim(value, "\n"); strings.Contains(trimmedTypedValue, "\n") {
				tmp := map[string]interface{}{key: value}
				tmp = parseMultilineBlockBody(tmp)
				newVal, _ := json.Marshal(tmp[key])
				value = string(newVal)
			}

			jsonData := pkgcommon.VariableToJSON(key, value)

			if err := ctx.JSONContext.AddJSON(jsonData); err != nil {
				return err
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
			}
		}
	}
	return nil
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
	if err := ctx.JSONContext.AddJSONObject(imageData); err != nil {
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
	return map[string]interface{}{
		entry.Name: imageData,
	}, nil
}

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
	manifest, err := image.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest for image reference: %s, error: %v", ref, err)
	}
	config, err := image.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config for image reference: %s, error: %v", ref, err)
	}
	data := map[string]interface{}{
		"image":         ref,
		"resolvedImage": fmt.Sprintf("%s@%s", parsedRef.Context().Name(), desc.Digest.String()),
		"registry":      parsedRef.Context().RegistryStr(),
		"repository":    parsedRef.Context().RepositoryStr(),
		"identifier":    parsedRef.Identifier(),
		"manifest":      manifest,
		"configData":    config,
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
		err = ctx.JSONContext.AddJSON(jsonData)
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

	contextNamedData := make(map[string]interface{})
	contextNamedData[entry.Name] = results
	contextData, err := json.Marshal(contextNamedData)
	if err != nil {
		return fmt.Errorf("failed to marshall data %v for context entry %v: %v", contextNamedData, entry, err)
	}

	err = ctx.JSONContext.AddJSON(contextData)
	if err != nil {
		return fmt.Errorf("failed to add JMESPath (%s) results to context, error: %v", entry.APICall.JMESPath, err)
	}

	logger.Info("added APICall context entry", "data", contextNamedData)
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

	err = ctx.JSONContext.AddJSON(data)
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

	// update the unstructuredObj["data"] to delimit and split the string value (containing "\n") with "\n"
	unstructuredObj["data"] = parseMultilineBlockBody(unstructuredObj["data"].(map[string]interface{}))

	// extract configmap data
	contextData["data"] = unstructuredObj["data"]
	contextData["metadata"] = unstructuredObj["metadata"]
	contextNamedData := make(map[string]interface{})
	contextNamedData[entry.Name] = contextData
	data, err := json.Marshal(contextNamedData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap %s/%s: %v", namespace, name, err)
	}

	return data, nil
}

// parseMultilineBlockBody recursively iterates through a map and updates its values to a list of strings
// if it encounters a string value containing newline delimiters "\n" and not in PEM format. This is done to
// allow specifying a list with newlines. Since PEM format keys can also contain newlines, an additional check
// is performed to skip splitting those into an array.
func parseMultilineBlockBody(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		switch typedValue := v.(type) {
		case string:
			trimmedTypedValue := strings.Trim(typedValue, "\n")
			if !pemFormat(trimmedTypedValue) && strings.Contains(trimmedTypedValue, "\n") {
				m[k] = strings.Split(trimmedTypedValue, "\n")
			} else {
				m[k] = trimmedTypedValue // trimming a str if it has trailing newline characters
			}
		default:
			continue
		}
	}
	return m
}

// check for PEM header found in certs and public keys
func pemFormat(s string) bool {
	return strings.Contains(s, "-----BEGIN")
}
