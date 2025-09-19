package helm

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// ChartLoader handles loading of Helm charts from various sources
type ChartLoader struct{}

// NewChartLoader creates a new chart loader
func NewChartLoader() *ChartLoader {
    return &ChartLoader{}
}

// ValidateChartPath validates if the given path is a valid Helm chart
func (cl *ChartLoader) ValidateChartPath(chartPath string) error {
    chartPath = filepath.Clean(chartPath)
    
    // Check if path exists
    info, err := os.Stat(chartPath)
    if err != nil {
        return fmt.Errorf("chart path does not exist: %w", err)
    }
    
    if info.IsDir() {
        // For directories, check for Chart.yaml
        chartFile := filepath.Join(chartPath, "Chart.yaml")
        if _, err := os.Stat(chartFile); err != nil {
            // Try Chart.yml as fallback
            chartFile = filepath.Join(chartPath, "Chart.yml")
            if _, err := os.Stat(chartFile); err != nil {
                return fmt.Errorf("directory %s does not contain a Chart.yaml or Chart.yml file", chartPath)
            }
        }
    } else {
        // For files, check if it's a valid chart archive
        if !strings.HasSuffix(chartPath, ".tgz") && !strings.HasSuffix(chartPath, ".tar.gz") {
            return fmt.Errorf("file %s is not a valid chart archive (.tgz or .tar.gz)", chartPath)
        }
    }
    
    return nil
}
