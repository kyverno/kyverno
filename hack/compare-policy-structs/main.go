package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type policyTypeComparison struct {
	name       string
	sourcePath string
	sourceAPI  string
	targetPath string
	targetAPI  string
	structName string
}

func main() {
	allGood := true

	comparisons := []policyTypeComparison{
		// Policy: v1 -> v2beta1
		{
			name:       "Policy",
			sourcePath: "api/kyverno/v1/policy_types.go",
			sourceAPI:  "v1",
			targetPath: "api/kyverno/v2beta1/policy_types.go",
			targetAPI:  "v2beta1",
			structName: "Policy",
		},
		// ClusterPolicy: v1 -> v2beta1
		{
			name:       "ClusterPolicy",
			sourcePath: "api/kyverno/v1/clusterpolicy_types.go",
			sourceAPI:  "v1",
			targetPath: "api/kyverno/v2beta1/clusterpolicy_types.go",
			targetAPI:  "v2beta1",
			structName: "ClusterPolicy",
		},
		// CleanupPolicy: v2 -> v2beta1
		{
			name:       "CleanupPolicy",
			sourcePath: "api/kyverno/v2/cleanup_policy_types.go",
			sourceAPI:  "v2",
			targetPath: "api/kyverno/v2beta1/cleanup_policy_types.go",
			targetAPI:  "v2beta1",
			structName: "CleanupPolicy",
		},
		// ClusterCleanupPolicy: v2 -> v2beta1
		{
			name:       "ClusterCleanupPolicy",
			sourcePath: "api/kyverno/v2/cleanup_policy_types.go",
			sourceAPI:  "v2",
			targetPath: "api/kyverno/v2beta1/cleanup_policy_types.go",
			targetAPI:  "v2beta1",
			structName: "ClusterCleanupPolicy",
		},
		// PolicyException: v2 -> v2beta1
		{
			name:       "PolicyException",
			sourcePath: "api/kyverno/v2/policy_exception_types.go",
			sourceAPI:  "v2",
			targetPath: "api/kyverno/v2beta1/policy_exception_types.go",
			targetAPI:  "v2beta1",
			structName: "PolicyException",
		},
	}

	for _, comp := range comparisons {
		fmt.Printf("\n=== Comparing %s (%s -> %s) ===\n", comp.name, comp.sourceAPI, comp.targetAPI)

		sourceFields := getFieldSet(comp.sourcePath, comp.structName)
		targetFields := getFieldSet(comp.targetPath, comp.structName)

		fmt.Printf("Checking for missing fields in %s.%s\n", comp.targetAPI, comp.structName)
		if !compareFields(sourceFields, targetFields, comp.sourceAPI+"."+comp.structName, comp.targetAPI+"."+comp.structName) {
			allGood = false
		}
	}

	fmt.Println("\n=== Comparison complete ===")
	if !allGood {
		fmt.Println("❌ Some fields are missing in target versions")
		os.Exit(1)
	} else {
		fmt.Println("✓ All fields are present in target versions")
	}
}

func compareFields(sourceFields, targetFields map[string]struct{}, sourceName, targetName string) bool {
	allPresent := true
	for field := range sourceFields {
		if _, ok := targetFields[field]; !ok {
			fmt.Printf("[MISSING] Field %q exists in %s but not in %s\n", field, sourceName, targetName)
			allPresent = false
		}
	}
	if allPresent {
		fmt.Printf("✓ All fields present\n")
	}
	return allPresent
}

func getFieldSet(filepath string, structName string) map[string]struct{} {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Failed to parse file: %v\n", err)
		os.Exit(1)
	}

	fields := make(map[string]struct{})

	for _, decl := range node.Decls {
		genDcl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDcl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				if field.Doc != nil {
					for _, comment := range field.Doc.List {
						if strings.Contains(strings.ToLower(comment.Text), "deprecated") {
							goto skip
						}
					}
				}
				fields[field.Names[0].Name] = struct{}{}
			skip:
			}
		}
	}
	return fields
}
