package test

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTestGitSourceSchemeCase(t *testing.T) {
	const testFile = `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: scheme-case
policies:
- policy.yaml
resources:
- resources.yaml
`
	tests := []struct {
		name       string
		path       string
		gitBranch  string
		wantBranch string
		wantFiles  []string
	}{{
		name:       "lower case scheme with branch in path",
		path:       "https://github.com/kyverno/policies/release-1.0",
		wantBranch: "release-1.0",
		wantFiles:  []string{"/best-practices/kyverno-test.yaml", "/kyverno-test.yaml"},
	}, {
		name:       "upper case scheme with branch in path",
		path:       "HTTPS://github.com/kyverno/policies/release-1.0",
		wantBranch: "release-1.0",
		wantFiles:  []string{"/best-practices/kyverno-test.yaml", "/kyverno-test.yaml"},
	}, {
		name:       "mixed case scheme with directory and branch flag",
		path:       "HtTpS://github.com/kyverno/policies/best-practices/",
		gitBranch:  "main",
		wantBranch: "main",
		wantFiles:  []string{"/best-practices/kyverno-test.yaml"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotURL, gotBranch string
			orig := cloneRepo
			t.Cleanup(func() { cloneRepo = orig })
			cloneRepo = func(url string, fs billy.Filesystem, branch string, _ http.BasicAuth) (*git.Repository, error) {
				gotURL, gotBranch = url, branch
				for _, dir := range []string{"/", "/best-practices/"} {
					if err := util.WriteFile(fs, dir+"kyverno-test.yaml", []byte(testFile), 0o644); err != nil {
						return nil, err
					}
				}
				return nil, nil
			}

			tests, err := loadTest(tt.path, "kyverno-test.yaml", tt.gitBranch)
			require.NoError(t, err)
			assert.Equal(t, "https://github.com/kyverno/policies", gotURL)
			assert.Equal(t, tt.wantBranch, gotBranch)
			var gotFiles []string
			for _, tc := range tests {
				require.NoError(t, tc.Err)
				gotFiles = append(gotFiles, tc.Path)
			}
			assert.Equal(t, tt.wantFiles, gotFiles)
		})
	}
}
