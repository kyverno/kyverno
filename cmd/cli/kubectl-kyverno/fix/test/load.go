package test

import (
	"os"
	"path/filepath"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type testCase struct {
	path string
	test *api.Test
	err  error
}

func loadTests(dirPath string, fileName string) ([]testCase, error) {
	return loadLocalTest(filepath.Clean(dirPath), fileName)
}

func loadLocalTest(path string, fileName string) ([]testCase, error) {
	var tests []testCase
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			ps, err := loadLocalTest(filepath.Join(path, file.Name()), fileName)
			if err != nil {
				return nil, err
			}
			tests = append(tests, ps...)
		} else if file.Name() == fileName {
			tests = append(tests, loadTest(path, file.Name()))
		}
	}
	return tests, nil
}

func loadTest(dirPath string, fileName string) testCase {
	path := filepath.Join(dirPath, fileName)
	yamlBytes, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return testCase{
			path: path,
			err:  err,
		}
	}
	var test api.Test
	if err := yaml.UnmarshalStrict(yamlBytes, &test); err != nil {
		return testCase{
			path: path,
			err:  err,
		}
	}
	return testCase{
		path: path,
		test: &test,
	}
}
