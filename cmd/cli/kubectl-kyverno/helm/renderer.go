package helm

import (
    "context"
    "fmt"
    "path/filepath"
    "strings"
    
    "helm.sh/helm/v3/pkg/action"
    "helm.sh/helm/v3/pkg/chart"
    "helm.sh/helm/v3/pkg/chart/loader"
    "helm.sh/helm/v3/pkg/cli"
    "helm.sh/helm/v3/pkg/engine"
    "helm.sh/helm/v3/pkg/chartutil"
    "helm.sh/helm/v3/pkg/strvals"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/util/yaml"
)

// Renderer handles Helm chart templating
type Renderer struct {
    settings *cli.EnvSettings
    config   *action.Configuration
}

// NewRenderer creates a new Helm renderer
func NewRenderer() (*Renderer, error) {
    settings := cli.New()
    
    return &Renderer{
        settings: settings,
    }, nil
}

// RenderChart renders a Helm chart to Kubernetes resources
func (r *Renderer) RenderChart(ctx context.Context, config *HelmConfig) (*ChartResult, error) {
    // Load the chart
    chart, err := r.loadChart(config.ChartPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load chart: %w", err)
    }
    
    // Process values
    values, err := r.processValues(config)
    if err != nil {
        return nil, fmt.Errorf("failed to process values: %w", err)
    }
    
    // Set up template options
    options := chartutil.ReleaseOptions{
        Name:      config.ReleaseName,
        Namespace: config.Namespace,
        IsInstall: true,
    }
    
    // Add built-in values
    valuesToRender, err := chartutil.ToRenderValues(chart, values, options, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare values: %w", err)
    }
    
    // Render templates
    rendered, err := engine.Render(chart, valuesToRender)
    if err != nil {
        return nil, fmt.Errorf("failed to render chart: %w", err)
    }
    
    // Process rendered templates
    result := &ChartResult{
        Chart:     chart,
        Resources: []RenderedResource{},
        Hooks:     []RenderedResource{},
    }
    
    for name, content := range rendered {
        if strings.HasSuffix(name, "NOTES.txt") {
            result.Notes = content
            continue
        }
        
        // Skip empty files
        if strings.TrimSpace(content) == "" {
            continue
        }
        
        resource := RenderedResource{
            Name:    name,
            Content: content,
            Source:  name,
        }
        
        // Determine if this is a hook or regular resource
        if r.isHook(content) {
            resource.Kind = "Hook"
            result.Hooks = append(result.Hooks, resource)
        } else {
            // Extract kind from the resource
            resource.Kind = r.extractKind(content)
            result.Resources = append(result.Resources, resource)
        }
    }
    
    return result, nil
}

// ConvertToUnstructured converts rendered resources to unstructured.Unstructured
func (r *Renderer) ConvertToUnstructured(resources []RenderedResource) ([]*unstructured.Unstructured, error) {
    var result []*unstructured.Unstructured
    
    for _, resource := range resources {
        // Split multi-document YAML
        documents := strings.Split(resource.Content, "---")
        
        for _, doc := range documents {
            doc = strings.TrimSpace(doc)
            if doc == "" {
                continue
            }
            
            // Parse YAML to unstructured
            obj := &unstructured.Unstructured{}
            if err := yaml.Unmarshal([]byte(doc), obj); err != nil {
                return nil, fmt.Errorf("failed to unmarshal resource %s: %w", resource.Name, err)
            }
            
            // Skip empty objects
            if len(obj.Object) == 0 {
                continue
            }
            
            result = append(result, obj)
        }
    }
    
    return result, nil
}

// loadChart loads a Helm chart from the given path
func (r *Renderer) loadChart(chartPath string) (*chart.Chart, error) {
    chartPath = filepath.Clean(chartPath)
    
    chart, err := loader.Load(chartPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load chart from %s: %w", chartPath, err)
    }
    
    return chart, nil
}

// processValues processes and merges values from various sources
func (r *Renderer) processValues(config *HelmConfig) (map[string]interface{}, error) {
    base := map[string]interface{}{}
    
    // Load values files
    for _, valuesFile := range config.ValuesFiles {
        vals, err := chartutil.ReadValuesFile(valuesFile)
        if err != nil {
            return nil, fmt.Errorf("failed to read values file %s: %w", valuesFile, err)
        }
        base = r.mergeMaps(base, vals.AsMap())
    }
    
    // Process --set values
    for _, set := range config.SetValues {
        if err := strvals.ParseInto(set, base); err != nil {
            return nil, fmt.Errorf("failed to parse --set value %s: %w", set, err)
        }
    }
    
    // Process --set-string values
    for _, set := range config.SetStringValues {
        if err := strvals.ParseIntoString(set, base); err != nil {
            return nil, fmt.Errorf("failed to parse --set-string value %s: %w", set, err)
        }
    }
    
    return base, nil
}

// Additional helper methods...
func (r *Renderer) isHook(content string) bool {
    return strings.Contains(content, "helm.sh/hook")
}

func (r *Renderer) extractKind(content string) string {
    lines := strings.Split(content, "\n")
    for _, line := range lines {
        if strings.HasPrefix(strings.TrimSpace(line), "kind:") {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 {
                return strings.TrimSpace(parts[1])
            }
        }
    }
    return "Unknown"
}

func (r *Renderer) mergeMaps(dst, src map[string]interface{}) map[string]interface{} {
    for key, srcVal := range src {
        if dstVal, exists := dst[key]; exists {
            if srcMap, srcIsMap := srcVal.(map[string]interface{}); srcIsMap {
                if dstMap, dstIsMap := dstVal.(map[string]interface{}); dstIsMap {
                    dst[key] = r.mergeMaps(dstMap, srcMap)
                    continue
                }
            }
        }
        dst[key] = srcVal
    }
    return dst
}