package test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type options struct {
	fileName string
	save     bool
	force    bool
	compress bool
}

func (o options) validate(dirs ...string) error {
	if o.fileName == "" {
		return errors.New("file-name must not be set to an empty string")
	}
	if len(dirs) == 0 {
		return errors.New("at least one test directory is required")
	}
	return nil
}

func (o options) execute(out io.Writer, dirs ...string) error {
	var testCases []test.TestCase
	for _, arg := range dirs {
		tests, err := test.LoadTests(arg, o.fileName)
		if err != nil {
			return err
		}
		testCases = append(testCases, tests...)
	}
	for _, testCase := range testCases {
		fmt.Fprintf(out, "Processing test file (%s)...", testCase.Path)
		fmt.Fprintln(out)
		if testCase.Err != nil {
			fmt.Fprintf(out, "  ERROR: loading test file (%s): %s", testCase.Path, testCase.Err)
			fmt.Fprintln(out)
			continue
		}
		fixed := *testCase.Test
		if fixed.ObjectMeta.Name == "" && fixed.Name == "" {
			fmt.Fprintln(out, "  WARNING: name is not set")
			fixed.ObjectMeta.Name = filepath.Base(testCase.Path)
		}
		fixed, messages, err := fix.FixTest(fixed, o.compress)
		for _, warning := range messages {
			fmt.Fprintln(out, "  WARNING:", warning)
		}
		if err != nil {
			fmt.Fprintln(out, "  ERROR:", err)
			continue
		}
		needsSave := !reflect.DeepEqual(testCase.Test, &fixed)
		if o.save && (o.force || needsSave) {
			fmt.Fprintf(out, "  Saving test file (%s)...", testCase.Path)
			fmt.Fprintln(out)
			untyped, err := kubeutils.ObjToUnstructured(fixed)
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to unstructured: %s", err)
				fmt.Fprintln(out)
				continue
			}
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "generation")
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "uid")
			jsonBytes, err := untyped.MarshalJSON()
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to json: %s", err)
				fmt.Fprintln(out)
				continue
			}
			yamlBytes, err := yaml.JSONToYAML(jsonBytes)
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to yaml: %s", err)
				fmt.Fprintln(out)
				continue
			}
			if err := os.WriteFile(testCase.Path, yamlBytes, os.ModePerm); err != nil {
				fmt.Fprintf(out, "    ERROR: saving test file (%s): %s", testCase.Path, err)
				fmt.Fprintln(out)
				continue
			}
			fmt.Fprintln(out, "    OK")
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, "Done.")
	return nil
}
