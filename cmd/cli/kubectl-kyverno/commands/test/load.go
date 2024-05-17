package test

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
)

func loadTests(paths []string, fileName string, gitBranch string) (test.TestCases, error) {
	var tests []test.TestCase
	for _, path := range paths {
		t, err := loadTest(path, fileName, gitBranch)
		if err != nil {
			return nil, err
		}
		tests = append(tests, t...)
	}
	return tests, nil
}

func loadTest(path string, fileName string, gitBranch string) (test.TestCases, error) {
	var tests []test.TestCase
	if source.IsGit(path) {
		fs := memfs.New()
		gitURL, err := url.Parse(path)
		if err != nil {
			return nil, err
		} else {
			pathElems := strings.Split(gitURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				return nil, fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch (without --git-branch flag) OR https://github.com/:owner/:repository/:directory (with --git-branch flag)", gitURL.Path)
			}
			gitURL.Path = strings.Join([]string{pathElems[0], pathElems[1]}, "/")
			repoURL := gitURL.String()
			var gitPathToYamls string
			if gitBranch == "" {
				gitPathToYamls = "/"
				if string(path[len(path)-1]) == "/" {
					gitBranch = strings.ReplaceAll(path, repoURL+"/", "")
				} else {
					gitBranch = strings.ReplaceAll(path, repoURL, "")
				}
				if gitBranch == "" {
					gitBranch = "main"
				} else if string(gitBranch[0]) == "/" {
					gitBranch = gitBranch[1:]
				}
			} else {
				if string(path[len(path)-1]) == "/" {
					gitPathToYamls = strings.ReplaceAll(path, repoURL+"/", "/")
				} else {
					gitPathToYamls = strings.ReplaceAll(path, repoURL, "/")
				}
			}
			if _, err := gitutils.Clone(repoURL, fs, gitBranch); err != nil {
				return nil, fmt.Errorf("Error: failed to clone repository \nCause: %s\n", err)
			}
			yamlFiles, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, fmt.Errorf("failed to list YAMLs in repository (%w)", err)
			}
			sort.Strings(yamlFiles)
			for _, yamlFilePath := range yamlFiles {
				if filepath.Base(yamlFilePath) == fileName {
					// resoucePath := strings.Trim(yamlFilePath, fileName)
					tests = append(tests, test.LoadTest(fs, yamlFilePath))
				}
			}
		}
		return tests, nil
	} else {
		tests, err := test.LoadTests(path, fileName)
		return tests, err
	}
}
