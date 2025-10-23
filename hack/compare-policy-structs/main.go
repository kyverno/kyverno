package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func main() {
	v1Fields := getFieldSet("api/kyverno/v1/policy_types.go", "Policy")
	v2beta1Fields := getFieldSet("api/kyverno/v2beta1/policy_types.go", "Policy")

	fmt.Println("Checking for missing fields in v2beta1")

	for field := range v1Fields {
		if _, ok := v2beta1Fields[field]; !ok {
			fmt.Printf("[MISSING] Field %q exists in v1 but not in v2beta1\n", field)
		}
	}
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
