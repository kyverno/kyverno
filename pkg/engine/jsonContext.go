package engine

import (
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/go-logr/logr"
	"github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/client-go/dynamic/dynamiclister"
)

// LoadContext - Fetches and adds external data to the Context.
func LoadContext(logger logr.Logger, contextEntries []kyverno.ContextEntry, resCache resourcecache.ResourceCache, ctx *PolicyContext, mockVariables map[string]map[string]string) error {
	if len(contextEntries) == 0 {
		return nil
	}

	// get GVR Cache for "configmaps"
	// can get cache for other resources if the informers are enabled in resource cache
	gvrC, ok := resCache.GetGVRCache("ConfigMap")
	if !ok {
		return errors.New("configmaps GVR Cache not found")
	}

	lister := gvrC.Lister()

	for _, entry := range contextEntries {
		if entry.ConfigMap != nil {
			if len(mockVariables) == 0 {
				mockVariables = make(map[string]map[string]string)
			}
			if err := loadConfigMap(logger, entry, lister, ctx.JSONContext, mockVariables); err != nil {
				return err
			}
		} else if entry.APICall != nil {
			if err := loadAPIData(logger, entry, ctx); err != nil {
				return err
			}
		}
	}

	return nil
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

	results, err := applyJMESPath(path.(string), jsonData)
	if err != nil {
		return fmt.Errorf("failed to apply JMESPath for context entry %v: %v", entry, err)
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

func applyJMESPath(jmesPath string, jsonData []byte) (interface{}, error) {
	jp, err := jmespath.Compile(jmesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", jmesPath, err)
	}

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %v", string(jsonData), err)
	}

	return jp.Search(data)
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

func loadConfigMap(logger logr.Logger, entry kyverno.ContextEntry, lister dynamiclister.Lister, ctx *context.Context, mockVariables map[string]map[string]string) error {
	if len(mockVariables) > 0 {
		for rule, variables := range mockVariables {
			if rule == "rule name" {
				// put these variables in the context
				for key, value := range variables {
					var subString string
					splitBySlash := strings.Split(key, "\"")
					if len(splitBySlash) > 1 {
						subString = splitBySlash[1]
					}

					startString := ""
					endString := ""
					lenOfVariableString := 0
					addedSlashString := false
					for _, k := range strings.Split(splitBySlash[0], ".") {
						if k != "" {
							startString += fmt.Sprintf(`{"%s":`, k)
							endString += `}`
							lenOfVariableString = lenOfVariableString + len(k) + 1
							if lenOfVariableString >= len(splitBySlash[0]) && len(splitBySlash) > 1 && addedSlashString == false {
								startString += fmt.Sprintf(`{"%s":`, subString)
								endString += `}`
								addedSlashString = true
							}
						}
					}

					midString := fmt.Sprintf(`"%s"`, value)
					finalString := startString + midString + endString
					var jsonData = []byte(finalString)
					ctx.AddJSON(jsonData)
				}
			}
		}

	} else {
		data, err := fetchConfigMap(logger, entry, lister, ctx)
		fmt.Println("\n**************************************")
		fmt.Println(string(data))
		fmt.Println("**************************************")
		if err != nil {
			return fmt.Errorf("failed to retrieve config map for context entry %s: %v", entry.Name, err)
		}

		err = ctx.AddJSON(data)
		if err != nil {
			return fmt.Errorf("failed to add config map for context entry %s: %v", entry.Name, err)
		}
	}

	return nil
}

func fetchConfigMap(logger logr.Logger, entry kyverno.ContextEntry, lister dynamiclister.Lister, jsonContext *context.Context) ([]byte, error) {
	contextData := make(map[string]interface{})

	name, err := variables.SubstituteAll(logger, jsonContext, entry.ConfigMap.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.name %s: %v", entry.Name, entry.ConfigMap.Name, err)
	}

	namespace, err := variables.SubstituteAll(logger, jsonContext, entry.ConfigMap.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context %s configMap.namespace %s: %v", entry.Name, entry.ConfigMap.Namespace, err)
	}

	if namespace == "" {
		namespace = "default"
	}

	key := fmt.Sprintf("%s/%s", namespace, name)
	obj, err := lister.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read configmap %s/%s from cache: %v", namespace, name, err)
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

// parseMultilineBlockBody recursively iterates through a map and updates its values in the following way
// whenever it encounters a string value containing "\n",
// it converts it into a []string by splitting it by "\n"
func parseMultilineBlockBody(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		switch typedValue := v.(type) {
		case string:
			trimmedTypedValue := strings.Trim(typedValue, "\n")
			if strings.Contains(trimmedTypedValue, "\n") {
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
