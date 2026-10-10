package common

import (
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5"
	git "github.com/go-git/go-git/v5"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var policyNamespaceSelector = []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "enforce-pod-name"
	},
	"spec": {
	  "validationFailureAction": "audit",
	  "background": true,
	  "rules": [
		{
		  "name": "validate-name",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ],
			  "namespaceSelector": {
				"matchExpressions": [
				  {
					"key": "foo.com/managed-state",
					"operator": "In",
					"values": [
					  "managed"
					]
				  }
				]
			  }
			}
		  },
		  "validate": {
			"message": "The Pod must end with -nginx",
			"pattern": {
			  "metadata": {
				"name": "*-nginx"
			  }
			}
		  }
		}
	  ]
	}
  }
`)

func Test_GetGitBranchOrPolicyPaths(t *testing.T) {
	type TestCase struct {
		gitBranch                             string
		repoURL                               string
		policyPath                            string
		desiredBranch, actualBranch           string
		desiredPathToYAMLs, actualPathToYAMLs string
	}
	testcases := []TestCase{
		{
			gitBranch:          "main",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/openshift/team-validate-ns-name/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/openshift/team-validate-ns-name/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
	}

	for _, tc := range testcases {
		tc.actualBranch, tc.actualPathToYAMLs = GetGitBranchOrPolicyPaths(tc.gitBranch, tc.repoURL, tc.policyPath)
		if tc.actualBranch != tc.desiredBranch || tc.actualPathToYAMLs != tc.desiredPathToYAMLs {
			t.Errorf("Want %q got %q  OR Want %q got %q", tc.desiredBranch, tc.actualBranch, tc.desiredPathToYAMLs, tc.actualPathToYAMLs)
		}
	}
}

// Test_ResolveGitSource_NestedGroups verifies that a repository living under
// nested groups/subgroups (as on GitLab) is resolved by trying the
// conventional <owner>/<repository> boundary first and widening it one path
// element at a time until a clone actually succeeds, rather than guessing
// the boundary from URL syntax. See kyverno/kyverno#16925.
func Test_ResolveGitSource_NestedGroups(t *testing.T) {
	const realRepoURL = "https://gitlab.example.com/group/subgroup/team/project"
	var attemptedRepoURLs []string
	clone := func(repoURL string, fs billy.Filesystem, branch string, auth githttp.BasicAuth) (*git.Repository, error) {
		attemptedRepoURLs = append(attemptedRepoURLs, repoURL)
		if repoURL != realRepoURL {
			return nil, fmt.Errorf("repository not found: %s", repoURL)
		}
		return nil, nil
	}

	fs, gitPathToYamls, branch, err := ResolveGitSource(realRepoURL+"/somedir/", "main", clone, githttp.BasicAuth{})
	assert.NilError(t, err)
	assert.Assert(t, fs != nil)
	assert.Equal(t, "/somedir/", gitPathToYamls)
	assert.Equal(t, "main", branch)
	assert.DeepEqual(t, []string{
		"https://gitlab.example.com/group/subgroup",
		"https://gitlab.example.com/group/subgroup/team",
		realRepoURL,
	}, attemptedRepoURLs)
}

// Test_ResolveGitSource_FlatLayout verifies that a conventional flat
// <owner>/<repository> layout (e.g. GitHub) resolves on the first attempt,
// with no widening, so existing behavior is unchanged.
func Test_ResolveGitSource_FlatLayout(t *testing.T) {
	var attemptedRepoURLs []string
	clone := func(repoURL string, fs billy.Filesystem, branch string, auth githttp.BasicAuth) (*git.Repository, error) {
		attemptedRepoURLs = append(attemptedRepoURLs, repoURL)
		return nil, nil
	}

	fs, gitPathToYamls, branch, err := ResolveGitSource("https://github.com/kyverno/policies/openshift/team-validate-ns-name/", "main", clone, githttp.BasicAuth{})
	assert.NilError(t, err)
	assert.Assert(t, fs != nil)
	assert.Equal(t, "/openshift/team-validate-ns-name/", gitPathToYamls)
	assert.Equal(t, "main", branch)
	assert.DeepEqual(t, []string{"https://github.com/kyverno/policies"}, attemptedRepoURLs)
}

// Test_ResolveGitSource_AllAttemptsFail verifies the final clone error is
// surfaced when no candidate boundary succeeds.
func Test_ResolveGitSource_AllAttemptsFail(t *testing.T) {
	clone := func(repoURL string, fs billy.Filesystem, branch string, auth githttp.BasicAuth) (*git.Repository, error) {
		return nil, fmt.Errorf("boom: %s", repoURL)
	}
	fs, _, _, err := ResolveGitSource("https://github.com/kyverno/policies/main", "", clone, githttp.BasicAuth{})
	assert.Assert(t, fs == nil)
	assert.ErrorContains(t, err, "boom")
}

func Test_getSubresourceKind(t *testing.T) {
	podAPIResource := metav1.APIResource{Name: "pods", SingularName: "", Namespaced: true, Kind: "Pod"}
	podEvictionAPIResource := metav1.APIResource{Name: "pods/eviction", SingularName: "", Namespaced: true, Group: "policy", Version: "v1", Kind: "Eviction"}

	subresources := []v1alpha1.Subresource{
		{
			Subresource:    podEvictionAPIResource,
			ParentResource: podAPIResource,
		},
	}

	subresourceKind, err := getSubresourceKind("", "Pod", "eviction", subresources)
	assert.NilError(t, err)
	assert.Equal(t, subresourceKind, "Eviction")
}
