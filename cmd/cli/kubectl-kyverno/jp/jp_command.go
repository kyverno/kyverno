package jp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gojmespath "github.com/jmespath/go-jmespath"
	jmespath "github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var applyHelp = `
For more information visit: https://kyverno.io/docs/writing-policies/jmespath/ 
`

// Command returns jp command
func Command() *cobra.Command {
	var compact, unquoted, ast, listFunctions bool
	var filename, exprFile string
	cmd := &cobra.Command{
		Use:          "jp",
		Short:        "Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions",
		SilenceUsage: true,
		Example:      applyHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFunctions {
				printFunctionList()
				return nil
			}
			// The following function has been adapted from
			// https://github.com/jmespath/jp/blob/54882e03bd277fc4475a677fab1d35eaa478b839/jp.go
			var expression string
			if exprFile != "" {
				byteExpr, err := os.ReadFile(filepath.Clean(exprFile))
				if err != nil {
					return fmt.Errorf("error opening expression file: %w", err)
				}
				expression = string(byteExpr)
			} else {
				if len(args) == 0 {
					return fmt.Errorf("must provide at least one argument")
				}
				expression = args[0]
			}
			if ast {
				parser := gojmespath.NewParser()
				parsed, err := parser.Parse(expression)
				if err != nil {
					if syntaxError, ok := err.(gojmespath.SyntaxError); ok {
						return fmt.Errorf("%w\n%s",
							syntaxError,
							syntaxError.HighlightLocation())
					}
					return err
				}
				fmt.Printf("%s", parsed)
				return nil
			}
			var input interface{}
			if filename != "" {
				f, err := os.ReadFile(filepath.Clean(filename))
				if err != nil {
					return fmt.Errorf("error opening input file: %w", err)
				}
				if err := yaml.Unmarshal(f, &input); err != nil {
					return fmt.Errorf("error parsing input json: %w", err)
				}
			} else {
				f, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("error opening input file: %w", err)
				}
				if err := yaml.Unmarshal(f, &input); err != nil {
					return fmt.Errorf("error parsing input json: %w", err)
				}
			}
			jp, err := jmespath.New(expression)
			if err != nil {
				return fmt.Errorf("failed to compile JMESPath: %s, error: %v", expression, err)
			}
			result, err := jp.Search(input)
			if err != nil {
				if syntaxError, ok := err.(gojmespath.SyntaxError); ok {
					return fmt.Errorf("%s\n%s",
						syntaxError,
						syntaxError.HighlightLocation())
				}
				return fmt.Errorf("error evaluating JMESPath expression: %w", err)
			}
			converted, isString := result.(string)
			if unquoted && isString {
				fmt.Println(converted)
			} else {
				var toJSON []byte
				if compact {
					toJSON, err = json.Marshal(result)
				} else {
					toJSON, err = json.MarshalIndent(result, "", "  ")
				}
				if err != nil {
					return fmt.Errorf("error marshalling result to JSON: %w", err)
				}
				fmt.Println(string(toJSON))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&compact, "compact", "c", false, "Produce compact JSON output that omits nonessential whitespace")
	cmd.Flags().BoolVarP(&listFunctions, "list-functions", "l", false, "Output a list of custom JMESPath functions in Kyverno")
	cmd.Flags().BoolVarP(&unquoted, "unquoted", "u", false, "If the final result is a string, it will be printed without quotes")
	cmd.Flags().BoolVar(&ast, "ast", false, "Only print the AST of the parsed expression.  Do not rely on this output, only useful for debugging purposes")
	cmd.Flags().StringVarP(&exprFile, "expr-file", "e", "", "Read JMESPath expression from the specified file")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "Read input from a JSON or YAML file instead of stdin")
	return cmd
}

func printFunctionList() {
	functions := []string{}
	for _, function := range jmespath.GetFunctions() {
		functions = append(functions, function.String())
	}
	sort.Strings(functions)
	fmt.Println(strings.Join(functions, "\n"))
}
