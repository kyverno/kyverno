package parse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gojmespath "github.com/jmespath/go-jmespath"
	"github.com/spf13/cobra"
)

var description = []string{
	"Parses jmespath expression and shows corresponding AST",
	"For more information visit: https://kyverno.io/docs/writing-policies/jmespath/ ",
}

var examples = []string{
	"  # Parse expression            \n  kyverno jp parse 'request.object.metadata.name | truncate(@, `9`)'",
	"  # Parse expression from a file\n  kyverno jp parse -f my-file",
	"  # Parse expression from stdin \n  kyverno jp parse",
	"  # Parse multiple expressionxs \n  kyverno jp parse -f my-file1 -f my-file-2 'request.object.metadata.name | truncate(@, `9`)'",
	"  # Cat into                    \n  cat my-file | kyverno jp parse",
}

func Command() *cobra.Command {
	var files []string
	cmd := &cobra.Command{
		Use:          "parse [-f file|expression]...",
		Short:        description[0],
		Long:         strings.Join(description, "\n"),
		Example:      strings.Join(examples, "\n\n"),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			expressions, err := loadExpressions(cmd, args, files)
			if err != nil {
				return err
			}
			for _, expression := range expressions {
				if err := printAst(expression); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&files, "file", "f", nil, "Read input from a JSON or YAML file instead of stdin")
	return cmd
}

func readFile(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadFile(file string) (string, error) {
	reader, err := os.Open(filepath.Clean(file))
	if err != nil {
		return "", fmt.Errorf("failed open file %s: %v", file, err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()
	content, err := readFile(reader)
	if err != nil {
		return "", fmt.Errorf("failed read file %s: %v", file, err)
	}
	return content, nil
}

func loadExpressions(cmd *cobra.Command, args []string, files []string) ([]string, error) {
	var expressions []string
	expressions = append(expressions, args...)
	for _, file := range files {
		expression, err := loadFile(file)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, expression)
	}
	if len(expressions) == 0 {
		fmt.Println("Reading from terminal input.")
		fmt.Println("Enter a jmespatch expression and hit Ctrl+D.")
		data, err := readFile(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("failed to read file STDIN: %v", err)
		}
		expressions = append(expressions, data)
	}
	return expressions, nil
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
	fmt.Println("#", expression)
	fmt.Println(parsed)
	return nil
}
