package test

import (
	"fmt"
	"os"
	"path/filepath"

	testutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/test"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func Command() *cobra.Command {
	var fileName string
	var save bool
	cmd := &cobra.Command{
		Use:     "test",
		Short:   "Fix inconsistencies and deprecated usage in Kyverno test files.",
		Example: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			var testCases []testutils.TestCase
			for _, arg := range args {
				tests, err := testutils.LoadTests(arg, fileName)
				if err != nil {
					return err
				}
				testCases = append(testCases, tests...)
			}
			for _, testCase := range testCases {
				fmt.Printf("Processing test file (%s)...", testCase.Path)
				fmt.Println()
				if testCase.Err != nil {
					fmt.Printf("  ERROR: loading test file (%s): %s", testCase.Path, testCase.Err)
					fmt.Println()
					continue
				}
				test := testCase.Test
				needsSave := false
				if test.Name == "" {
					fmt.Println("  WARNING: name is not set")
					test.Name = filepath.Base(testCase.Path)
					needsSave = true
				}
				if len(test.Policies) == 0 {
					fmt.Println("  WARNING: test has no policies")
				}
				if len(test.Resources) == 0 {
					fmt.Println("  WARNING: test has no policies")
				}
				for i := range test.Results {
					result := &test.Results[i]
					if result.Resource != "" && len(result.Resources) != 0 {
						fmt.Println("  WARNING: test result should not use both `resource` and `resources` fields")
					}
					if result.Resource != "" {
						fmt.Println("  WARNING: test result uses deprecated `resource` field, moving it into the `resources` field")
						result.Resources = append(result.Resources, result.Resource)
						result.Resource = ""
						needsSave = true
					}
					if result.Status != "" && result.Result != "" {
						fmt.Println("  ERROR: test result should not use both `status` and `result` fields")
					}
					if result.Status != "" && result.Result == "" {
						fmt.Println("  WARNING: test result uses deprecated `status` field, moving it into the `result` field")
						result.Result = result.Status
						result.Status = ""
						needsSave = true
					}
				}
				if save && needsSave {
					fmt.Printf("  Saving test file (%s)...", testCase.Path)
					fmt.Println()
					yamlBytes, err := yaml.Marshal(test)
					if err != nil {
						fmt.Printf("    ERROR: converting test to yaml: %s", err)
						fmt.Println()
						continue
					}
					if err := os.WriteFile(testCase.Path, yamlBytes, os.ModePerm); err != nil {
						fmt.Printf("    ERROR: saving test file (%s): %s", testCase.Path, err)
						fmt.Println()
						continue
					}
					fmt.Println("    OK")
				}
				fmt.Println()
			}
			fmt.Println("Done.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "kyverno-test.yaml", "Test filename")
	cmd.Flags().BoolVar(&save, "save", false, "Save fixed file")
	return cmd
}
