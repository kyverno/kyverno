package test

import (
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
)

// TestLoadTest_GitNestedGroupRepository verifies that a git source URL whose
// repository lives under nested groups/subgroups (as on GitLab) is resolved
// correctly: the CLI is expected to try the conventional <owner>/<repository>
// boundary first, and only widen it, one path element at a time, until a
// clone actually succeeds - it must not guess the boundary from URL syntax
// alone. See https://github.com/kyverno/kyverno/issues/16925.
func TestLoadTest_GitNestedGroupRepository(t *testing.T) {
	const realRepoURL = "https://gitlab.example.com/group/subgroup/team/project"
	const testCaseYaml = `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: nested-group-test
policies:
- policy.yaml
resources:
- resource.yaml
results: []
`

	var attemptedRepoURLs []string
	originalCloneFunc := cloneFunc
	cloneFunc = func(repoURL string, fs billy.Filesystem, branch string, auth http.BasicAuth) (*git.Repository, error) {
		attemptedRepoURLs = append(attemptedRepoURLs, repoURL)
		if repoURL != realRepoURL {
			return nil, fmt.Errorf("repository not found: %s", repoURL)
		}
		if err := util.WriteFile(fs, "/somedir/kyverno-test.yaml", []byte(testCaseYaml), 0o644); err != nil {
			return nil, err
		}
		return nil, nil
	}
	defer func() { cloneFunc = originalCloneFunc }()

	tests, err := loadTest(realRepoURL+"/somedir/", "kyverno-test.yaml", "main")
	assert.NoError(t, err)
	assert.Len(t, tests, 1)
	assert.Equal(t, "nested-group-test", tests[0].Test.ObjectMeta.Name)

	// The conventional two-element boundary, and every intermediate
	// boundary, must be tried - and fail - before the real (nested)
	// repository boundary is finally attempted and succeeds.
	assert.Equal(t, []string{
		"https://gitlab.example.com/group/subgroup",
		"https://gitlab.example.com/group/subgroup/team",
		realRepoURL,
	}, attemptedRepoURLs)
}
