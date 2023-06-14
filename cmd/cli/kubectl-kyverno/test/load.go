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
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type policy struct {
	bytes        []byte
	resourcePath string
}

func loadTests(
	dirPath []string,
	fileName string,
	gitBranch string,
) (billy.Filesystem, []policy, []error) {
	var policies []policy
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
			if policyYamls, err := gitutils.ListYamls(fs, gitPathToYamls); err != nil {
				errors = append(errors, sanitizederror.NewWithError("failed to list YAMLs in repository", err))
			} else {
				sort.Strings(policyYamls)
				for _, yamlFilePath := range policyYamls {
					file, err := fs.Open(yamlFilePath)
					if err != nil {
						errors = append(errors, sanitizederror.NewWithError("Error: failed to open file", err))
						continue
					}
					if path.Base(file.Name()) == fileName {
						policyresoucePath := strings.Trim(yamlFilePath, fileName)
						bytes, err := io.ReadAll(file)
						if err != nil {
							errors = append(errors, sanitizederror.NewWithError("Error: failed to read file", err))
							continue
						}
						policyBytes, err := yaml.ToJSON(bytes)
						if err != nil {
							errors = append(errors, sanitizederror.NewWithError("failed to convert to JSON", err))
							continue
						}
						policies = append(policies, policy{
							bytes:        policyBytes,
							resourcePath: policyresoucePath,
						})
					}
				}
			}
		}
		return fs, policies, errors
	} else {
		path := filepath.Clean(dirPath[0])
		policies, errors = loadLocalTest(path, fileName)
		return nil, policies, errors
	}
}

func loadLocalTest(
	path string,
	fileName string,
) ([]policy, []error) {
	var policies []policy
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
				yamlFile, err := os.ReadFile(filepath.Join(path, file.Name())) // #nosec G304
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("unable to read yaml", err))
					continue
				}
				valuesBytes, err := yaml.ToJSON(yamlFile)
				if err != nil {
					errors = append(errors, sanitizederror.NewWithError("failed to convert json", err))
					continue
				}
				policies = append(policies, policy{
					bytes:        valuesBytes,
					resourcePath: path,
				})
			}
		}
	}
	return policies, errors
}
