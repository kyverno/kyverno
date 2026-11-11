package apply

import (
	"context"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPoliciesGitSourceSchemeCase(t *testing.T) {
	const policy = `apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: has(object.metadata.labels)
`
	tests := []struct {
		name       string
		path       string
		wantURL    string
		wantBranch string
	}{{
		name:       "lower case scheme",
		path:       "https://github.com/kyverno/policies/release-1.0",
		wantURL:    "https://github.com/kyverno/policies",
		wantBranch: "release-1.0",
	}, {
		name:       "upper case scheme",
		path:       "HTTPS://github.com/kyverno/policies/release-1.0",
		wantURL:    "https://github.com/kyverno/policies",
		wantBranch: "release-1.0",
	}, {
		name:       "mixed case scheme",
		path:       "HtTp://github.com/kyverno/policies/release-1.0",
		wantURL:    "http://github.com/kyverno/policies",
		wantBranch: "release-1.0",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotURL, gotBranch string
			c := &ApplyCommandConfig{
				PolicyPaths: []string{tt.path},
				Cloner: func(url string, fs billy.Filesystem, branch string, _ http.BasicAuth) (*git.Repository, error) {
					gotURL, gotBranch = url, branch
					return nil, util.WriteFile(fs, "/policy.yaml", []byte(policy), 0o644)
				},
			}
			_, _, _, _, _, vps, _, _, _, _, _, _, err := c.loadPolicies(context.Background(), io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, gotURL)
			assert.Equal(t, tt.wantBranch, gotBranch)
			require.Len(t, vps, 1)
			assert.Equal(t, "require-labels", vps[0].GetName())
		})
	}
}
