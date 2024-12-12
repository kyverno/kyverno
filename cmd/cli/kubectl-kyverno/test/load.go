package test

import (
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func LoadTests(dirPath string, fileName string) (TestCases, error) {
	return loadLocalTest(filepath.Clean(dirPath), fileName)
}

func loadLocalTest(path string, fileName string) (TestCases, error) {
	var tests []TestCase
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
			tests = append(tests, LoadTest(nil, filepath.Join(path, fileName)))
		}
	}
	return tests, nil
}

func LoadTest(fs billy.Filesystem, path string) TestCase {
	var yamlBytes []byte
	if fs != nil {
		file, err := fs.Open(path)
		if err != nil {
			return TestCase{
				Path: path,
				Fs:   fs,
				Err:  err,
			}
		}
		data, err := io.ReadAll(file)
		if err != nil {
			return TestCase{
				Path: path,
				Fs:   fs,
				Err:  err,
			}
		}
		yamlBytes = data
	} else {
		data, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			return TestCase{
				Path: path,
				Fs:   fs,
				Err:  err,
			}
		}
		yamlBytes = data
	}
	var test v1alpha1.Test
	if err := yaml.UnmarshalStrict(yamlBytes, &test); err != nil {
		return TestCase{
			Path: path,
			Fs:   fs,
			Err:  err,
		}
	}
	cleanTest(&test)
	return TestCase{
		Path: path,
		Fs:   fs,
		Test: &test,
	}
}

func cleanTest(test *v1alpha1.Test) {
	test.Policies = removeDuplicateStrings(test.Policies)
	test.Resources = removeDuplicateStrings(test.Resources)
	for index, result := range test.Results {
		stringArr := []string{}
		for _, res := range result.Resources {
			stringArr = append(stringArr, res.(string))
		}
		stringArr = removeDuplicateStrings(stringArr)

		anyArr := []any{}
		for _, res := range stringArr {
			stringArr = append(stringArr, res)
		}

		test.Results[index].Resources = anyArr
	}
}

func removeDuplicateStrings(strings []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, str := range strings {
		if _, exists := seen[str]; !exists {
			seen[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}
