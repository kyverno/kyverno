package helm

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// ValuesProcessor handles processing of Helm values
type ValuesProcessor struct{}

// NewValuesProcessor creates a new values processor
func NewValuesProcessor() *ValuesProcessor {
    return &ValuesProcessor{}
}

// ValidateValuesFile validates if a file is a valid values file
func (vp *ValuesProcessor) ValidateValuesFile(valuesPath string) error {
    valuesPath = filepath.Clean(valuesPath)
    
    // Check if file exists
    info, err := os.Stat(valuesPath)
    if err != nil {
        return fmt.Errorf("values file does not exist: %w", err)
    }
    
    if info.IsDir() {
        return fmt.Errorf("values path %s is a directory, not a file", valuesPath)
    }
    
    // Check file extension
    ext := strings.ToLower(filepath.Ext(valuesPath))
    if ext != ".yaml" && ext != ".yml" {
        return fmt.Errorf("values file %s must have .yaml or .yml extension", valuesPath)
    }
    
    return nil
}