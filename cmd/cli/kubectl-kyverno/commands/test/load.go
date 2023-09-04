package test

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/source"
	testutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/test"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
)

func loadTests(dirPath []string, fileName string, gitBranch string) (billy.Filesystem, testutils.TestCases, error) {
	var tests []testutils.TestCase
	// TODO support multiple paths
	path := dirPath[0]
	if source.IsGit(path) {
		fs := memfs.New()
		gitURL, err := url.Parse(path)
		if err != nil {
			return nil, nil, err
		} else {
			pathElems := strings.Split(gitURL.Path[1:], "/")
			if len(pathElems) <= 1 {
				return nil, nil, fmt.Errorf("invalid URL path %s - expected https://github.com/:owner/:repository/:branch (without --git-branch flag) OR https://github.com/:owner/:repository/:directory (with --git-branch flag)", gitURL.Path)
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
				return nil, nil, fmt.Errorf("Error: failed to clone repository \nCause: %s\n", err)
			}
			yamlFiles, err := gitutils.ListYamls(fs, gitPathToYamls)
			if err != nil {
				return nil, nil, sanitizederror.NewWithError("failed to list YAMLs in repository", err)
			}
			sort.Strings(yamlFiles)
			for _, yamlFilePath := range yamlFiles {
				if filepath.Base(yamlFilePath) == fileName {
					// resoucePath := strings.Trim(yamlFilePath, fileName)
					tests = append(tests, testutils.LoadTest(fs, yamlFilePath))
				}
			}
		}
		return fs, tests, nil
	} else {
		tests, err := testutils.LoadTests(path, fileName)
		return nil, tests, err
	}
}
