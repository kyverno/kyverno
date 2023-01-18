package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gojmespath "github.com/jmespath/go-jmespath"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var description = []string{
	"Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions",
	"For more information visit: https://kyverno.io/docs/writing-policies/jmespath/ ",
}

var examples = []string{
	"  # Evaluate query             \n  kyverno jp query -i object.yaml 'request.object.metadata.name | truncate(@, `9`)'",
	"  # Evaluate query             \n  kyverno jp query -i object.yaml -q query-file",
	"  # Evaluate multiple queries  \n  kyverno jp query -i object.yaml -q query-file-1 -q query-file-2 'request.object.metadata.name | truncate(@, `9`)'",
	"  # Cat query into             \n  cat query-file | kyverno jp query -i object.yaml",
	"  # Cat object into            \n  cat object.yaml | kyverno jp query -q query-file",
}

// Command returns jp command
func Command() *cobra.Command {
	var compact, unquoted bool
	var input string
	var queries []string
	cmd := &cobra.Command{
		Use:          "query [-i input] [-q query|query]...",
		Short:        description[0],
		Long:         strings.Join(description, "\n"),
		SilenceUsage: true,
		Example:      strings.Join(examples, "\n\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			queries, err := loadQueries(args, queries)
			if err != nil {
				return err
			}
			input, err := loadInput(input)
			if err != nil {
				return err
			}
			if len(queries) == 0 && input == nil {
				return errors.New("at least one query or input object is required")
			}
			if len(queries) == 0 {
				query, err := readQuery(cmd)
				if err != nil {
					return err
				}
				queries = append(queries, query)
			}
			if input == nil {
				i, err := readInput(cmd)
				if err != nil {
					return err
				}
				input = i
			}
			for _, query := range queries {
				result, err := evaluate(input, query)
				if err != nil {
					return err
				}
				if err := printResult(query, result, unquoted, compact); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&compact, "compact", "c", false, "Produce compact JSON output that omits non essential whitespace")
	cmd.Flags().BoolVarP(&unquoted, "unquoted", "u", false, "If the final result is a string, it will be printed without quotes")
	cmd.Flags().StringSliceVarP(&queries, "query", "q", nil, "Read JMESPath expression from the specified file")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Read input from a JSON or YAML file instead of stdin")
	return cmd
}

func readFile(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func loadFile(file string) ([]byte, error) {
	reader, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, fmt.Errorf("failed open file %s: %v", file, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()
	content, err := readFile(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %v", file, err)
	}
	return content, nil
}

func readQuery(cmd *cobra.Command) (string, error) {
	fmt.Println("Reading from terminal input.")
	fmt.Println("Enter a jmespatch expression and hit Ctrl+D.")
	data, err := readFile(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadQueries(args []string, files []string) ([]string, error) {
	var queries []string
	queries = append(queries, args...)
	for _, file := range files {
		query, err := loadFile(file)
		if err != nil {
			return nil, err
		}
		queries = append(queries, string(query))
	}
	return queries, nil
}

func readInput(cmd *cobra.Command) (interface{}, error) {
	fmt.Println("Reading from terminal input.")
	fmt.Println("Enter input object and hit Ctrl+D.")
	data, err := readFile(cmd.InOrStdin())
	if err != nil {
		return nil, err
	}
	var input interface{}
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("error parsing input json: %w", err)
	}
	return input, nil
}

func loadInput(file string) (interface{}, error) {
	if file == "" {
		return nil, nil
	}
	data, err := loadFile(file)
	if err != nil {
		return nil, err
	}
	var input interface{}
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("error parsing input json: %w", err)
	}
	return input, nil
}

func evaluate(input interface{}, query string) (interface{}, error) {
	jp, err := jmespath.New(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", query, err)
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

func printResult(query string, result interface{}, unquoted bool, compact bool) error {
	converted, isString := result.(string)
	fmt.Println("#", query)
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
