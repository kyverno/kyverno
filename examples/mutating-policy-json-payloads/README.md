# MutatingPolicy JSON Payload Support

This example demonstrates how MutatingPolicy can now process JSON payloads that are not Kubernetes resources.

## Overview

The MutatingPolicy engine has been enhanced to support JSON payloads through the `JsonPayload` field in `EngineRequest`. This allows MutatingPolicy to process any JSON data, not just Kubernetes resources.

## Example Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/kyverno/kyverno/pkg/cel/engine"
    "github.com/kyverno/kyverno/pkg/cel/libs"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func main() {
    // Create a JSON payload (non-Kubernetes)
    jsonPayload := &unstructured.Unstructured{
        Object: map[string]interface{}{
            "user": map[string]interface{}{
                "name":  "john",
                "email": "john@example.com",
            },
            "settings": map[string]interface{}{
                "theme": "light",
            },
        },
    }

    // Create an engine request using the JSON payload
    request := engine.RequestFromJSON(contextProvider, jsonPayload)

    // Process with MutatingPolicy engine
    response, err := mpolEngine.Handle(context.Background(), request, nil)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Original: %v\n", response.Resource)
    if response.PatchedResource != nil {
        fmt.Printf("Patched: %v\n", response.PatchedResource)
    }
}
```

## Key Features

1. **Automatic Detection**: The engine automatically detects whether the request contains a Kubernetes admission request or a JSON payload
2. **Seamless Processing**: JSON payloads are processed through the same mutation pipeline as Kubernetes resources
3. **Policy Compatibility**: Existing MutatingPolicies can be applied to JSON payloads
4. **Minimal Overhead**: No changes required to existing policies or workflows

## Use Cases

- Processing non-Kubernetes JSON configurations
- Applying policies to API request/response payloads
- Validating and mutating custom data formats
- Integrating with external systems that produce JSON data

## Implementation Details

The implementation modifies the `Handle` and `MatchedMutateExistingPolicies` methods in the MutatingPolicy engine to:

1. Check if `request.JsonPayload` is provided
2. Use the JSON payload directly instead of extracting from admission request
3. Create minimal admission attributes for policy evaluation
4. Process mutations using the same CEL evaluation engine
