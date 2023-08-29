package test

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	api "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api/test/legacy"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type testCase struct {
	test         *api.Test
	resourcePath string
}

func loadTests(
	dirPath []string,
	fileName string,
	gitBranch string,
) (billy.Filesystem, []testCase, []error) {
	var tests []testCase
	var errors []error
	if strings.Contains(dirPath[0], "https://") {
		fs := memfs.New()
		if gitURL, err := url.Parse(dirPath[0]); err != nil {
			errors = append(errors, sanitizederror.NewWithError("failed to parse URL", err))
		} else {
			pathElems := strings.Split(gitURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				err := fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch (without --git-branch flag) OR https://github.com/:owner/:repository/:directory (with --git-branch flag)", gitURL.Path)
				fmt.Printf("Error: failed to parse URL \nCause: %s\n", err)
				os.Exit(1)
			}
			gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitURL.String()
			var gitPathToYamls string
			if gitBranch == "" {
				gitPathToYamls = "/"
				if string(dirPath[0][len(dirPath[0])-1]) == "/" {
					gitBranch = strings.ReplaceAll(dirPath[0], repoURL+"/", "")
				} else {
					gitBranch = strings.ReplaceAll(dirPath[0], repoURL, "")
				}
				if gitBranch == "" {
					gitBranch = "main"
				} else if string(gitBranch[0]) == "/" {
					gitBranch = gitBranch[1:]
				}
			} else {
				if string(dirPath[0][len(dirPath[0])-1]) == "/" {
					gitPathToYamls = strings.ReplaceAll(dirPath[0], repoURL+"/", "/")
				} else {
					gitPathToYamls = strings.ReplaceAll(dirPath[0], repoURL, "/")
				}
			}
			_, cloneErr := gitutils.Clone(repoURL, fs, gitBranch)
			if cloneErr != nil {
				fmt.Printf("Error: failed to clone repository \nCause: %s\n", cloneErr)
				log.Log.V(3).Info(fmt.Sprintf("failed to clone repository  %v as it is not valid", repoURL), "error", cloneErr)
				os.Exit(1)
			}
			if yamlFiles, err := gitutils.ListYamls(fs, gitPathToYamls); err != nil {
				errors = append(errors, sanitizederror.NewWithError("failed to list YAMLs in repository", err))
			} else {
				sort.Strings(yamlFiles)
				for _, yamlFilePath := range yamlFiles {
					file, err := fs.Open(yamlFilePath)
					if err != nil {
						errors = append(errors, sanitizederror.NewWithError("Error: failed to open file", err))
						continue
					}
					if path.Base(file.Name()) == fileName {
						resoucePath := strings.Trim(yamlFilePath, fileName)
						yamlBytes, err := io.ReadAll(file)
						if err != nil {
							errors = append(errors, fmt.Errorf("failed to read file (%s)", err))
							continue
						}
						test, err := loadTest(yamlBytes)
						if err != nil {
							errors = append(errors, fmt.Errorf("failed to load test file (%s)", err))
							continue
						}
						tests = append(tests, testCase{
							test:         test,
							resourcePath: resoucePath,
						})
					}
				}
			}
		}
		return fs, tests, errors
	} else {
		path := filepath.Clean(dirPath[0])
		tests, errors = loadLocalTest(path, fileName)
		return nil, tests, errors
	}
}

func loadLocalTest(
	path string,
	fileName string,
) ([]testCase, []error) {
	var policies []testCase
	var errors []error
	files, err := os.ReadDir(path)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to read %v: %v", path, err.Error()))
	} else {
		for _, file := range files {
			if file.IsDir() {
				ps, errs := loadLocalTest(filepath.Join(path, file.Name()), fileName)
				policies = append(policies, ps...)
				errors = append(errors, errs...)
			} else if file.Name() == fileName {
				// We accept the risk of including files here as we read the test dir only.
				yamlBytes, err := os.ReadFile(filepath.Join(path, file.Name())) // #nosec G304
				if err != nil {
					errors = append(errors, fmt.Errorf("unable to read yaml (%s)", err))
					continue
				}
				test, err := loadTest(yamlBytes)
				if err != nil {
					errors = append(errors, fmt.Errorf("failed to load test file (%s)", err))
					continue
				}
				policies = append(policies, testCase{
					test:         test,
					resourcePath: path,
				})
			}
		}
	}
	return policies, errors
}

func loadTest(data []byte) (*api.Test, error) {
	var test api.Test
	if err := yaml.UnmarshalStrict(data, &test); err != nil {
		return nil, err
	}
	return &test, nil
}
