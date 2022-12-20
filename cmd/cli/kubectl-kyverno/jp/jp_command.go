package jp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	gojmespath "github.com/jmespath/go-jmespath"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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
			} else {
				expression, err := loadExpression(exprFile, args)
				if err != nil {
					return err
				}
				if ast {
					return printAst(expression)
				} else {
					input, err := loadInput(filename)
					if err != nil {
						return err
					}
					result, err := evaluate(expression, input)
					if err != nil {
						return err
					}
					return printResult(result, unquoted, compact)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&compact, "compact", "c", false, "Produce compact JSON output that omits non essential whitespace")
	cmd.Flags().BoolVarP(&listFunctions, "list-functions", "l", false, "Output a list of custom JMESPath functions in Kyverno")
	cmd.Flags().BoolVarP(&unquoted, "unquoted", "u", false, "If the final result is a string, it will be printed without quotes")
	cmd.Flags().BoolVar(&ast, "ast", false, "Only print the AST of the parsed expression.  Do not rely on this output, only useful for debugging purposes")
	cmd.Flags().StringVarP(&exprFile, "expr-file", "e", "", "Read JMESPath expression from the specified file")
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "Read input from a JSON or YAML file instead of stdin")
	return cmd
}

func loadExpression(file string, args []string) (string, error) {
	if file != "" {
		data, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return "", fmt.Errorf("error opening expression file: %w", err)
		}
		return string(data), nil
	} else {
		if len(args) == 0 {
			return "", fmt.Errorf("must provide at least one argument")
		}
		return args[0], nil
	}
}

func loadInput(file string) (interface{}, error) {
	var data []byte
	if file != "" {
		f, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return nil, fmt.Errorf("error opening input file: %w", err)
		}
		data = f
	} else {
		f, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("error opening input file: %w", err)
		}
		data = f
	}
	var input interface{}
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("error parsing input json: %w", err)
	}
	return input, nil
}

func evaluate(expression string, input interface{}) (interface{}, error) {
	jp, err := jmespath.New(expression)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", expression, err)
	}
	result, err := jp.Search(input)
	if err != nil {
		if syntaxError, ok := err.(gojmespath.SyntaxError); ok {
			return nil, fmt.Errorf("%s\n%s", syntaxError, syntaxError.HighlightLocation())
		}
		return nil, fmt.Errorf("error evaluating JMESPath expression: %w", err)
	}
	return result, nil
}

func printResult(result interface{}, unquoted bool, compact bool) error {
	converted, isString := result.(string)
	if unquoted && isString {
		fmt.Println(converted)
	} else {
		var toJSON []byte
		var err error
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
}

func printFunctionList() {
	functions := jmespath.GetFunctions()
	slices.SortFunc(functions, func(a, b *jmespath.FunctionEntry) bool {
		return a.String() < b.String()
	})
	for _, function := range functions {
		function := *function
		note := function.Note
		function.Note = ""
		fmt.Println("Name:", function.Entry.Name)
		fmt.Println("  Signature:", function.String())
		if note != "" {
			fmt.Println("  Note:     ", note)
		}
		fmt.Println()
	}
}

// The following function has been adapted from
// https://github.com/jmespath/jp/blob/54882e03bd277fc4475a677fab1d35eaa478b839/jp.go
func printAst(expression string) error {
	parser := gojmespath.NewParser()
	parsed, err := parser.Parse(expression)
	if err != nil {
		if syntaxError, ok := err.(gojmespath.SyntaxError); ok {
			return fmt.Errorf("%w\n%s", syntaxError, syntaxError.HighlightLocation())
		}
		return err
	}
	fmt.Print(parsed)
	return nil
}
