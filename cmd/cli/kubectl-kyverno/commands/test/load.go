package test

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
)

// cloneFunc performs the actual git clone for loadTest. It is a variable so
// tests can replace it to avoid real network calls while still exercising
// the full git-URL loading code path.
var cloneFunc gitutils.CloneFunc = gitutils.Clone

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
		var auth http.BasicAuth
		fs, gitPathToYamls, _, err := common.ResolveGitSource(path, gitBranch, cloneFunc, auth)
		if err != nil {
			return nil, fmt.Errorf("error: failed to clone repository \nCause: %s", err)
		}
		yamlFiles, err := gitutils.ListYamls(fs, gitPathToYamls)
		if err != nil {
			return nil, fmt.Errorf("error: failed to list YAMLs in repository (%w)", err)
		}
		sort.Strings(yamlFiles)
		for _, yamlFilePath := range yamlFiles {
			if filepath.Base(yamlFilePath) == fileName {
				// resourcePath := strings.Trim(yamlFilePath, fileName)
				tests = append(tests, test.LoadTest(fs, yamlFilePath)...)
			}
		}
		return tests, nil
	} else {
		tests, err := test.LoadTests(path, fileName)
		return tests, err
	}
}
