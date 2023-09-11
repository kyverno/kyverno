package test

import (
	"fmt"
	"os"
	"path/filepath"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func Command() *cobra.Command {
	var fileName string
	var save bool
	var compress bool
	cmd := &cobra.Command{
		Use:     "test [folder]...",
		Short:   command.FormatDescription(true, websiteUrl, true, description...),
		Long:    command.FormatDescription(false, websiteUrl, true, description...),
		Example: command.FormatExamples(examples...),
		RunE: func(cmd *cobra.Command, args []string) error {
			var testCases []test.TestCase
			for _, arg := range args {
				tests, err := test.LoadTests(arg, fileName)
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
				if compress {
					compressed := map[key][]string{}
					for _, result := range test.Results {
						k := key{
							Policy:                      result.Policy,
							Rule:                        result.Rule,
							IsValidatingAdmissionPolicy: result.IsValidatingAdmissionPolicy,
							Result:                      result.Result,
							Kind:                        result.Kind,
							Namespace:                   result.Namespace,
							PatchedResource:             result.PatchedResource,
							GeneratedResource:           result.GeneratedResource,
							CloneSourceResource:         result.CloneSourceResource,
						}
						compressed[k] = append(compressed[k], result.Resources...)
					}
					if len(compressed) != len(test.Results) {
						needsSave = true
					}
					test.Results = nil
					for k, v := range compressed {
						test.Results = append(test.Results, testapi.TestResults{
							Policy:                      k.Policy,
							Rule:                        k.Rule,
							IsValidatingAdmissionPolicy: k.IsValidatingAdmissionPolicy,
							Result:                      k.Result,
							Kind:                        k.Kind,
							Namespace:                   k.Namespace,
							PatchedResource:             k.PatchedResource,
							GeneratedResource:           k.GeneratedResource,
							CloneSourceResource:         k.CloneSourceResource,
							Resources:                   v,
						})
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
	cmd.Flags().BoolVar(&compress, "compress", false, "Compress test results")
	return cmd
}

type key struct {
	Policy                      string
	Rule                        string
	IsValidatingAdmissionPolicy bool
	Result                      policyreportv1alpha2.PolicyResult
	Kind                        string
	Namespace                   string
	PatchedResource             string
	GeneratedResource           string
	CloneSourceResource         string
}
