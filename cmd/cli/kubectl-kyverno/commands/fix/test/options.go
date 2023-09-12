package test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	testapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"sigs.k8s.io/yaml"
)

type options struct {
	fileName string
	save     bool
	compress bool
}

func (o options) validate() error {
	if o.fileName == "" {
		return errors.New("file-name must not be set to an empty string")
	}
	return nil
}

func (o options) execute(args ...string) error {
	var testCases []test.TestCase
	for _, arg := range args {
		tests, err := test.LoadTests(arg, o.fileName)
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
			fmt.Println("  WARNING: test has no resources")
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
			if result.Namespace != "" {
				fmt.Println("  WARNING: test result uses deprecated `namespace` field, replacing `policy` with a `<namespace>/<name>` pattern")
				result.Policy = fmt.Sprintf("%s/%s", result.Namespace, result.Policy)
				result.Namespace = ""
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
		if o.compress {
			compressed := map[testapi.TestResultBase][]string{}
			for _, result := range test.Results {
				compressed[result.TestResultBase] = append(compressed[result.TestResultBase], result.Resources...)
			}
			if len(compressed) != len(test.Results) {
				needsSave = true
			}
			test.Results = nil
			for k, v := range compressed {
				test.Results = append(test.Results, testapi.TestResult{
					TestResultBase: k,
					Resources:      v,
				})
			}
		}
		if o.save && needsSave {
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
}
